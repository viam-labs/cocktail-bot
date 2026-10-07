"""Table plane from the camera point cloud, fitted in the camera frame.

Assumes the table is the dominant plane within table_max_depth_mm of the camera.
"""

import numpy as np
from numpy.typing import NDArray

from .types import Plane

MAX_POINTS = 50_000
RANSAC_ITERATIONS = 1000
RANSAC_BATCH = 50
MIN_POINTS = 500
MIN_INLIER_RATIO = 0.05
MIN_OPTICAL_AXIS_ANGLE_DEG = 5.0


class TableNotFoundError(RuntimeError):
    pass


def fit_table_plane(
    points_mm: NDArray[np.float64],
    max_depth_mm: float,
    ransac_threshold_mm: float,
    offset_mm: float = 0.0,
    seed: int = 0,
) -> Plane:
    rng = np.random.default_rng(seed)
    points = np.asarray(points_mm, dtype=np.float64)
    keep = np.all(np.isfinite(points), axis=1) & (points[:, 2] > 0.0) & (points[:, 2] <= max_depth_mm)
    points = points[keep]
    if points.shape[0] < MIN_POINTS:
        raise TableNotFoundError(f"only {points.shape[0]} valid cloud points within {max_depth_mm} mm")
    if points.shape[0] > MAX_POINTS:
        points = points[rng.choice(points.shape[0], MAX_POINTS, replace=False)]

    best_count = -1
    best_normal = np.zeros(3)
    best_d = 0.0
    for _ in range(RANSAC_ITERATIONS // RANSAC_BATCH):
        samples = points[rng.integers(0, points.shape[0], size=(RANSAC_BATCH, 3))]
        normals = np.cross(samples[:, 1] - samples[:, 0], samples[:, 2] - samples[:, 0])
        norms = np.linalg.norm(normals, axis=1)
        valid = norms > 1e-9
        if not np.any(valid):
            continue
        normals = normals[valid] / norms[valid, None]
        ds = -np.einsum("ij,ij->i", normals, samples[valid, 0])
        counts = (np.abs(points @ normals.T + ds) < ransac_threshold_mm).sum(axis=0)
        i = int(np.argmax(counts))
        if counts[i] > best_count:
            best_count = int(counts[i])
            best_normal, best_d = normals[i], float(ds[i])

    normal, d = best_normal, best_d
    for _ in range(2):
        inliers = points[np.abs(points @ normal + d) < ransac_threshold_mm]
        if inliers.shape[0] < 3:
            raise TableNotFoundError("RANSAC found no plane")
        centroid = inliers.mean(axis=0)
        normal = np.linalg.svd(inliers - centroid, full_matrices=False)[2][-1]
        d = -float(normal @ centroid)

    inlier_ratio = float(np.mean(np.abs(points @ normal + d) < ransac_threshold_mm))
    if inlier_ratio < MIN_INLIER_RATIO:
        raise TableNotFoundError(f"best plane has only {inlier_ratio:.1%} inliers")
    if d < 0.0:
        normal, d = -normal, -d
    if abs(normal[2]) < np.sin(np.deg2rad(MIN_OPTICAL_AXIS_ANGLE_DEG)):
        raise TableNotFoundError(f"plane normal {normal.round(3).tolist()} is nearly perpendicular to the optical axis")
    return Plane(normal=normal, d_mm=d - offset_mm, inlier_ratio=inlier_ratio)
