import numpy as np
import torch
from numpy.typing import NDArray

from spill_glass_finder.keypoints import CROP_SIZE


def gaussian_heatmaps(
    peaks_uv: list[NDArray[np.float64] | None], n_channels: int = 5, sigma: float = 3.0
) -> torch.Tensor:
    """(n_channels, 256, 256) heatmaps peaked at each crop-space (u, v); None leaves that channel empty."""
    v, u = torch.meshgrid(
        torch.arange(CROP_SIZE, dtype=torch.float32), torch.arange(CROP_SIZE, dtype=torch.float32), indexing="ij"
    )
    maps = torch.zeros(n_channels, CROP_SIZE, CROP_SIZE)
    for c, peak in enumerate(peaks_uv):
        if peak is not None:
            maps[c] = 0.9 * torch.exp(-((u - float(peak[0])) ** 2 + (v - float(peak[1])) ** 2) / (2 * sigma**2))
    return maps


class StubHeatmapModel:
    """Returns fixed per-crop heatmaps in batch order and records the inputs it saw."""

    def __init__(self, per_crop_peaks: list[list[NDArray[np.float64] | None]]) -> None:
        self.heatmaps = torch.stack([gaussian_heatmaps(peaks) for peaks in per_crop_peaks])
        self.inputs: list[torch.Tensor] = []

    def __call__(self, x: torch.Tensor) -> torch.Tensor:
        self.inputs.append(x)
        assert x.shape[0] == self.heatmaps.shape[0]
        return self.heatmaps
