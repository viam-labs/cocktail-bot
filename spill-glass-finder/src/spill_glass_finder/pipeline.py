"""find_glasses: one RGB image + one point cloud + intrinsics + detector boxes -> fitted glasses. No Viam I/O."""

from dataclasses import dataclass

import numpy as np
from numpy.typing import NDArray
from viam.logging import getLogger

from .config import GlassFinderConfig
from .geometry import GlassSolveError, deduplicate, solve_glass
from .keypoints import HeatmapModel, detect_keypoints
from .table import fit_table_plane
from .types import BoxDetection, GlassEstimate, Intrinsics, Plane

logger = getLogger(__name__)


@dataclass(frozen=True)
class FindResult:
    glasses: list[GlassEstimate]
    table: Plane


def select_detections(
    detections: list[BoxDetection], labels: tuple[str, ...], min_confidence: float
) -> list[BoxDetection]:
    kept = [d for d in detections if d.label in labels and d.score >= min_confidence]
    return sorted(kept, key=lambda d: d.score, reverse=True)


def find_glasses(
    image_rgb: NDArray[np.uint8],
    cloud_mm: NDArray[np.float64],
    intrinsics: Intrinsics,
    detections: list[BoxDetection],
    model: HeatmapModel,
    min_keypoint_pixel_distance: int,
    config: GlassFinderConfig,
) -> FindResult:
    image_height, image_width = image_rgb.shape[:2]
    if (image_width, image_height) != (intrinsics.width_px, intrinsics.height_px):
        raise ValueError(
            f"image is {image_width}x{image_height} but the camera intrinsics are for "
            f"{intrinsics.width_px}x{intrinsics.height_px}"
        )
    table = fit_table_plane(
        cloud_mm, config.table_max_depth_mm, config.table_ransac_threshold_mm, config.table_offset_mm
    )
    candidates = select_detections(detections, config.labels, config.min_confidence)
    keypoints = detect_keypoints(
        image_rgb, candidates, model, config.crop_padding, config.channel_order, min_keypoint_pixel_distance
    )

    k = intrinsics.matrix()
    glasses: list[GlassEstimate] = []
    for detection, kp in zip(candidates, keypoints):
        if kp is None:
            logger.info("skipping %s (score %.2f): not all four keypoints found", detection.label, detection.score)
            continue
        try:
            glasses.append(solve_glass(detection, kp, k, table, config.tilt_deg))
        except GlassSolveError as err:
            logger.info("skipping %s (score %.2f): %s", detection.label, detection.score, err)
    return FindResult(glasses=deduplicate(glasses), table=table)


def points_in_box(
    cloud_mm: NDArray[np.float64], intrinsics: Intrinsics, box: tuple[int, int, int, int]
) -> NDArray[np.float64]:
    """Cloud points whose projection falls inside the pixel box; assumes depth is registered to the color image."""
    points = cloud_mm[np.all(np.isfinite(cloud_mm), axis=1) & (cloud_mm[:, 2] > 0.0)]
    u = intrinsics.fx * points[:, 0] / points[:, 2] + intrinsics.cx
    v = intrinsics.fy * points[:, 1] / points[:, 2] + intrinsics.cy
    x0, y0, x1, y1 = box
    return points[(u >= x0) & (u <= x1) & (v >= y0) & (v <= y1)]
