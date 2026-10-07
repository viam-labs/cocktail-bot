"""SPILL keypoint pre/post-processing: pad the detector box, crop, resize to 256x256, find heatmap peaks, map back."""

from typing import Callable, Literal

import cv2
import numpy as np
import torch
from numpy.typing import NDArray

from .model import get_keypoints_from_heatmap_batch_maxpool
from .types import BoxDetection, Keypoints2D

CROP_SIZE = 256
N_STRUCTURAL_KEYPOINTS = 4

HeatmapModel = Callable[[torch.Tensor], torch.Tensor]
ChannelOrder = Literal["rgb", "bgr"]


def padded_box(
    detection: BoxDetection, image_width: int, image_height: int, padding: float
) -> tuple[int, int, int, int]:
    width = detection.x_max - detection.x_min
    height = detection.y_max - detection.y_min
    pad_x = int(padding * width)
    pad_y = int(padding * height)
    return (
        max(0, detection.x_min - pad_x),
        max(0, detection.y_min - pad_y),
        min(image_width, detection.x_max + pad_x),
        min(image_height, detection.y_max + pad_y),
    )


def crop_and_resize(image: NDArray[np.uint8], box: tuple[int, int, int, int]) -> NDArray[np.uint8]:
    """Non-aspect-preserving resize, as SPILL does."""
    x1, y1, x2, y2 = box
    return cv2.resize(image[y1:y2, x1:x2], (CROP_SIZE, CROP_SIZE))


def crop_to_image(uv_crop: NDArray[np.float64], box: tuple[int, int, int, int]) -> NDArray[np.float64]:
    x1, y1, x2, y2 = box
    return np.array([uv_crop[0] / CROP_SIZE * (x2 - x1) + x1, uv_crop[1] / CROP_SIZE * (y2 - y1) + y1])


def to_model_input(crops: NDArray[np.uint8], channel_order: ChannelOrder) -> torch.Tensor:
    """(N, 256, 256, 3) RGB uint8 -> (N, 3, 256, 256) float in [0, 1] in the model's channel order."""
    if channel_order == "bgr":
        crops = crops[..., ::-1]
    return torch.from_numpy(np.ascontiguousarray(crops)).permute(0, 3, 1, 2).float() / 255.0


def detect_keypoints(
    image_rgb: NDArray[np.uint8],
    detections: list[BoxDetection],
    model: HeatmapModel,
    padding: float,
    channel_order: ChannelOrder,
    min_keypoint_pixel_distance: int,
) -> list[Keypoints2D | None]:
    """One batched forward pass; None for a detection missing any of the four structural keypoints."""
    if not detections:
        return []
    image_height, image_width = image_rgb.shape[:2]
    boxes = [padded_box(det, image_width, image_height, padding) for det in detections]
    crops = np.stack([crop_and_resize(image_rgb, box) for box in boxes])
    with torch.inference_mode():
        heatmaps = model(to_model_input(crops, channel_order))
    peaks = get_keypoints_from_heatmap_batch_maxpool(heatmaps, min_keypoint_pixel_distance=min_keypoint_pixel_distance)

    results: list[Keypoints2D | None] = []
    for box, channels in zip(boxes, peaks):
        if any(not channels[c] for c in range(N_STRUCTURAL_KEYPOINTS)):
            results.append(None)
            continue
        uv = [crop_to_image(np.asarray(channels[c][0], dtype=np.float64), box) for c in range(N_STRUCTURAL_KEYPOINTS)]
        results.append(Keypoints2D(bottom_front=uv[0], top_front=uv[1], top_left=uv[2], top_right=uv[3]))
    return results
