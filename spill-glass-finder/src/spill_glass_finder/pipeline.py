"""Runs SPILL's localize_table + localize_glass on one frame. Unit conversion happens here: Viam mm <-> SPILL m."""

from dataclasses import dataclass

import numpy as np
from numpy.typing import NDArray

from .config import GlassFinderConfig
from .detections import FrameDetections, select_detections
from .spill.glassloc import GlassLocalizer
from .types import BoxDetection, GlassEstimate, Intrinsics

PLATFORM_HEIGHT_M = 0.0


@dataclass(frozen=True)
class FindResult:
    glasses: list[GlassEstimate]
    table_height_mm: float


def find_glasses(
    localizer: GlassLocalizer,
    frame_detections: FrameDetections,
    image_bgr: NDArray[np.uint8],
    cloud_camera_mm: NDArray[np.float64],
    intrinsics: Intrinsics,
    detections: list[BoxDetection],
    config: GlassFinderConfig,
) -> FindResult:
    """Glasses in SPILL's order (dedup, then sorted by distance to platform point (0, 1 m, table height))."""
    image_height, image_width = image_bgr.shape[:2]
    if (image_width, image_height) != (intrinsics.width_px, intrinsics.height_px):
        raise ValueError(
            f"image is {image_width}x{image_height} but the camera intrinsics are for "
            f"{intrinsics.width_px}x{intrinsics.height_px}"
        )
    x_platform_camera = config.world_from_camera_m
    table_height = localizer.localize_table(
        cloud_camera_mm / 1000.0,
        x_platform_camera,
        PLATFORM_HEIGHT_M,
        config.table_crop_height_min_m,
        config.table_crop_height_max_m,
    )
    frame_detections.set_detections(select_detections(detections, config.labels, config.min_confidence))
    localizer.camera_intrinsics = intrinsics.matrix()
    spill_glasses = localizer.localize_glass(image_bgr, table_height, x_platform_camera, PLATFORM_HEIGHT_M)

    glasses = []
    for top_middle_3d, radius_3d, height_3d, glass_angle, bb, keypoints_o in spill_glasses:
        detection = frame_detections.match(bb)
        rim_center_platform = (x_platform_camera @ np.append(top_middle_3d, 1.0))[:3]
        glasses.append(
            GlassEstimate(
                label=detection.label,
                score=detection.score,
                bbox=(detection.x_min, detection.y_min, detection.x_max, detection.y_max),
                keypoints_px=np.asarray(keypoints_o[:4], dtype=np.float64),
                rim_center_mm=rim_center_platform * 1000.0,
                radius_mm=float(radius_3d) * 1000.0,
                height_mm=float(height_3d) * 1000.0,
                tilt_deg=float(np.rad2deg(glass_angle)),
            )
        )
    return FindResult(glasses=glasses, table_height_mm=float(table_height) * 1000.0)


def points_in_box(
    cloud_mm: NDArray[np.float64], intrinsics: Intrinsics, box: tuple[int, int, int, int]
) -> NDArray[np.float64]:
    """Cloud points whose projection falls inside the pixel box; assumes depth is registered to the color image."""
    points = cloud_mm[np.all(np.isfinite(cloud_mm), axis=1) & (cloud_mm[:, 2] > 0.0)]
    u = intrinsics.fx * points[:, 0] / points[:, 2] + intrinsics.cx
    v = intrinsics.fy * points[:, 1] / points[:, 2] + intrinsics.cy
    x0, y0, x1, y1 = box
    return points[(u >= x0) & (u <= x1) & (v >= y0) & (v <= y1)]


def to_world_mm(points_camera_mm: NDArray[np.float64], world_from_camera_m: NDArray[np.float64]) -> NDArray[np.float64]:
    return points_camera_mm @ world_from_camera_m[:3, :3].T + world_from_camera_m[:3, 3] * 1000.0


def sample_cylinder(
    rim_center_mm: NDArray[np.float64], radius_mm: float, height_mm: float, n_points: int
) -> NDArray[np.float64]:
    """World-frame points on an upright cylinder hanging from the rim center (SPILL's visualizer cylinder): 80% side
    wall, 20% rim circle."""
    rng = np.random.default_rng(0)
    n_rim = int(round(n_points * 0.2))
    depth = np.concatenate([rng.uniform(0.0, height_mm, n_points - n_rim), np.zeros(n_rim)])
    phi = rng.uniform(0.0, 2.0 * np.pi, n_points)
    return rim_center_mm + np.column_stack([radius_mm * np.cos(phi), radius_mm * np.sin(phi), -depth])
