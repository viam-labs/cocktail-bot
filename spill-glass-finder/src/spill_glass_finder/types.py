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
class GlassEstimate:
    """One glass from SPILL's localize_glass, converted to mm in the world frame."""

    label: str
    score: float
    bbox: tuple[int, int, int, int]
    keypoints_px: NDArray[np.float64]
    rim_center_mm: NDArray[np.float64]
    radius_mm: float
    height_mm: float
    tilt_deg: float

    def keypoints_dict(self) -> dict[str, list[float]]:
        return {name: [float(v) for v in uv] for name, uv in zip(KEYPOINT_NAMES, self.keypoints_px)}
