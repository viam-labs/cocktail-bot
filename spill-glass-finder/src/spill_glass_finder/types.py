from dataclasses import dataclass

import numpy as np
from numpy.typing import NDArray

KEYPOINT_NAMES: tuple[str, ...] = ("bottom_front", "top_front", "top_left", "top_right")


@dataclass(frozen=True)
class Intrinsics:
    width_px: int
    height_px: int
    fx: float
    fy: float
    cx: float
    cy: float

    def matrix(self) -> NDArray[np.float64]:
        return np.array([[self.fx, 0.0, self.cx], [0.0, self.fy, self.cy], [0.0, 0.0, 1.0]])


@dataclass(frozen=True)
class BoxDetection:
    label: str
    score: float
    x_min: int
    y_min: int
    x_max: int
    y_max: int


@dataclass(frozen=True)
class Keypoints2D:
    """Pixel coordinates (u, v) in the full image."""

    bottom_front: NDArray[np.float64]
    top_front: NDArray[np.float64]
    top_left: NDArray[np.float64]
    top_right: NDArray[np.float64]

    def as_dict(self) -> dict[str, list[float]]:
        return {name: [float(v) for v in getattr(self, name)] for name in KEYPOINT_NAMES}


@dataclass(frozen=True)
class Plane:
    """Table plane n·p + d = 0 in the camera frame (mm); n is unit and points up, toward the camera side."""

    normal: NDArray[np.float64]
    d_mm: float
    inlier_ratio: float


@dataclass(frozen=True)
class GlassEstimate:
    label: str
    score: float
    bbox: tuple[int, int, int, int]
    keypoints_px: Keypoints2D
    rim_center_mm: NDArray[np.float64]
    base_front_mm: NDArray[np.float64]
    up: NDArray[np.float64]
    back: NDArray[np.float64]
    radius_mm: float
    height_mm: float
    tilt_deg: float
    reprojection_rmse_px: float
