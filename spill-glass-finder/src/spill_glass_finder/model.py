"""MaxViT-UNet keypoint detector, inference only.

Vendored from tlpss/keypoint-detection (https://github.com/tlpss/keypoint-detection, MIT), which is not on PyPI and
pins pytorch-lightning<=1.9.4, wandb and fiftyone. Module and parameter names match the Lightning checkpoint's
state_dict, so wild_glasses.ckpt loads strictly.
"""

from pathlib import Path

import timm
import torch
import torch.nn as nn
from torchvision.models.feature_extraction import create_feature_extractor


class UpSamplingBlock(nn.Module):
    def __init__(self, n_channels_in: int, n_skip_channels_in: int, n_channels_out: int, kernel_size: int) -> None:
        super().__init__()
        self.conv1 = nn.Conv2d(
            in_channels=n_skip_channels_in + n_channels_in,
            out_channels=n_channels_out,
            kernel_size=kernel_size,
            bias=False,
            padding="same",
        )
        self.norm1 = nn.BatchNorm2d(n_channels_out)
        self.relu1 = nn.ReLU()

    def forward(self, x: torch.Tensor, x_skip: torch.Tensor) -> torch.Tensor:
        x = nn.functional.interpolate(x, scale_factor=2.0)
        x = torch.cat([x, x_skip], dim=1)
        return self.relu1(self.norm1(self.conv1(x)))


class MaxVitUnet(nn.Module):
    FEATURE_CHANNELS = (64, 64, 128, 256, 512)
    MODEL_NAME = "maxvit_nano_rw_256"
    FEATURE_LAYERS = ["stem", "stages.0", "stages.1", "stages.2", "stages.3"]

    def __init__(self) -> None:
        super().__init__()
        # pretrained=False: every weight comes from the checkpoint, so skip timm's ImageNet download.
        self.encoder = timm.create_model(self.MODEL_NAME, pretrained=False, num_classes=0)
        self.feature_extractor = create_feature_extractor(self.encoder, self.FEATURE_LAYERS)
        self.decoder_blocks = nn.ModuleList(
            UpSamplingBlock(c_in, c_skip, c_skip, 3)
            for c_skip, c_in in zip(self.FEATURE_CHANNELS, self.FEATURE_CHANNELS[1:])
        )
        # Unused in forward, but present in the checkpoint's state_dict.
        self.final_conv = nn.Conv2d(self.FEATURE_CHANNELS[0], self.FEATURE_CHANNELS[0], 3, padding="same")
        self.final_upsampling_block = UpSamplingBlock(self.FEATURE_CHANNELS[0], 3, self.FEATURE_CHANNELS[0], 3)

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        orig_x = torch.clone(x)
        features = list(self.feature_extractor(x).values())
        x = features.pop(-1)
        for block in self.decoder_blocks[::-1]:
            x = block(x, features.pop(-1))
        return self.final_upsampling_block(x, orig_x)


class KeypointDetector(nn.Module):
    def __init__(self, n_heatmaps: int) -> None:
        super().__init__()
        self.unnormalized_model = nn.Sequential(
            MaxVitUnet(),
            nn.Conv2d(MaxVitUnet.FEATURE_CHANNELS[0], n_heatmaps, kernel_size=(3, 3), padding="same"),
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        """(N, 3, H, W) in [0, 1] -> (N, n_heatmaps, H, W) heatmaps in [0, 1]."""
        return torch.sigmoid(self.unnormalized_model(x))


def load_keypoint_detector(checkpoint_path: str | Path) -> tuple[KeypointDetector, dict]:
    """Load the Lightning checkpoint on CPU; returns the eval-mode model and its hyper_parameters."""
    checkpoint = torch.load(checkpoint_path, map_location="cpu", weights_only=True)
    hparams = checkpoint["hyper_parameters"]
    model = KeypointDetector(n_heatmaps=len(hparams["keypoint_channel_configuration"]))
    model.load_state_dict(_drop_stale_index_buffers(checkpoint["state_dict"], model))
    model.eval()
    return model, hparams


def load_from_checkpoint(checkpoint_path: str | Path) -> KeypointDetector:
    """Stands in for keypoint_detection.utils.load_checkpoints.load_from_checkpoint, as SPILL calls it."""
    model, _ = load_keypoint_detector(checkpoint_path)
    return model


def _drop_stale_index_buffers(state_dict: dict[str, torch.Tensor], model: nn.Module) -> dict[str, torch.Tensor]:
    """Older timm saved MaxViT's relative_position_index as a persistent buffer; current timm recomputes it.

    Drop those keys, but only after checking they equal the recomputed buffers.
    """
    buffers = dict(model.named_buffers(remove_duplicate=False))
    persistent = model.state_dict().keys()
    cleaned: dict[str, torch.Tensor] = {}
    for key, value in state_dict.items():
        if key.endswith("relative_position_index") and key not in persistent:
            if key not in buffers or not torch.equal(buffers[key], value):
                raise ValueError(f"checkpoint buffer {key} does not match the model built by timm {timm.__version__}")
            continue
        cleaned[key] = value
    return cleaned


def get_keypoints_from_heatmap_batch_maxpool(
    heatmap: torch.Tensor,
    max_keypoints: int = 20,
    min_keypoint_pixel_distance: int = 1,
    abs_max_threshold: float | None = None,
    rel_max_threshold: float | None = None,
) -> list[list[list[list[int]]]]:
    """Vendored from keypoint_detection.utils.heatmap: [batch][channel] -> list of [u, v] peaks, best first."""
    batch_size, n_channels, _, width = heatmap.shape
    kernel = min_keypoint_pixel_distance * 2 + 1
    pad = min_keypoint_pixel_distance
    # Padding with 1.0 suppresses peaks within `pad` pixels of the border.
    padded_heatmap = torch.nn.functional.pad(heatmap, (pad, pad, pad, pad), mode="constant", value=1.0)
    max_pooled_heatmap = torch.nn.functional.max_pool2d(padded_heatmap, kernel, stride=1, padding=0)
    heatmap = heatmap * (max_pooled_heatmap == heatmap)

    scores, indices = torch.topk(heatmap.view(batch_size, n_channels, -1), max_keypoints, sorted=True)
    indices = torch.stack([torch.div(indices, width, rounding_mode="floor"), indices % width], dim=-1)
    indices_np = indices.detach().cpu().numpy()
    scores_np = scores.detach().cpu().numpy()

    threshold = 0.01
    if abs_max_threshold is not None:
        threshold = max(threshold, abs_max_threshold)
    if rel_max_threshold is not None:
        threshold = max(threshold, rel_max_threshold * float(heatmap.max()))

    return [
        [
            [indices_np[b, c, i][::-1].tolist() for i in range(indices_np.shape[2]) if scores_np[b, c, i] > threshold]
            for c in range(n_channels)
        ]
        for b in range(batch_size)
    ]
