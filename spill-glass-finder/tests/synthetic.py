from dataclasses import dataclass

import numpy as np
from numpy.typing import NDArray

from spill_glass_finder.geometry import glass_points, project
from spill_glass_finder.types import Intrinsics, Keypoints2D, Plane

INTRINSICS = Intrinsics(width_px=1280, height_px=720, fx=910.0, fy=905.0, cx=642.0, cy=358.0)


def camera_up(pitch_deg: float, roll_deg: float = 0.0) -> NDArray[np.float64]:
    """World up in the camera frame for a camera pitched down by pitch_deg and rolled about its optical axis."""
    pitch, roll = np.deg2rad(pitch_deg), np.deg2rad(roll_deg)
    up = np.array([0.0, -np.cos(pitch), -np.sin(pitch)])
    c, s = np.cos(roll), np.sin(roll)
    return np.array([[c, -s, 0.0], [s, c, 0.0], [0.0, 0.0, 1.0]]) @ up


@dataclass(frozen=True)
class SyntheticGlass:
    plane: Plane
    keypoints: Keypoints2D
    rim_center_mm: NDArray[np.float64]
    base_front_mm: NDArray[np.float64]
    radius_mm: float
    height_mm: float
    tilt_deg: float


def make_glass(
    up: NDArray[np.float64],
    base_front_px: tuple[float, float],
    range_mm: float,
    radius_mm: float,
    height_mm: float,
    tilt_deg: float,
    intrinsics: Intrinsics = INTRINSICS,
    noise_px: float = 0.0,
    rng: np.random.Generator | None = None,
) -> SyntheticGlass:
    """Glass whose base front point lies range_mm from the camera along the ray through base_front_px."""
    k = intrinsics.matrix()
    ray = np.linalg.solve(k, np.array([base_front_px[0], base_front_px[1], 1.0]))
    base_front = range_mm / 1000.0 * ray / np.linalg.norm(ray)
    d_m = -float(up @ base_front)
    assert d_m > 0.0, "base front ray must point down at the table"
    model = glass_points(base_front, up, radius_mm / 1000.0, height_mm / 1000.0, np.deg2rad(tilt_deg))
    uv = [project(k, p) for p in (model.base_front, model.top_front, model.top_left, model.top_right)]
    if noise_px > 0.0:
        assert rng is not None
        uv = [p + rng.normal(0.0, noise_px, 2) for p in uv]
    return SyntheticGlass(
        plane=Plane(normal=up, d_mm=d_m * 1000.0, inlier_ratio=1.0),
        keypoints=Keypoints2D(bottom_front=uv[0], top_front=uv[1], top_left=uv[2], top_right=uv[3]),
        rim_center_mm=model.rim_center * 1000.0,
        base_front_mm=model.base_front * 1000.0,
        radius_mm=radius_mm,
        height_mm=height_mm,
        tilt_deg=tilt_deg,
    )


def tabletop_cloud(up: NDArray[np.float64], height_mm: float, rng: np.random.Generator) -> NDArray[np.float64]:
    """Camera-frame cloud (mm): noisy table seen through every 4th pixel, three boxes on it, and random outliers."""
    u, v = np.meshgrid(np.arange(0, INTRINSICS.width_px, 4), np.arange(0, INTRINSICS.height_px, 4))
    rays = np.linalg.solve(INTRINSICS.matrix(), np.stack([u.ravel(), v.ravel(), np.ones(u.size)]).astype(float)).T
    t = -height_mm / (rays @ up)
    table = rays[t > 0] * t[t > 0, None]
    table = table[table[:, 2] < 1800.0]
    table += rng.normal(0.0, 1.5, table.shape) * (table[:, 2:3] / 1000.0)

    foot = -height_mm * up
    forward = np.array([0.0, 0.0, 1.0]) - up[2] * up
    forward /= np.linalg.norm(forward)
    side = np.cross(up, forward)
    clutter = []
    for along, across, size, tall in ((500, -100, 80, 200), (700, 150, 120, 90), (450, 200, 60, 300)):
        n = 3000
        local = rng.uniform([-size / 2, -size / 2, 0.0], [size / 2, size / 2, tall], (n, 3))
        clutter.append(
            foot
            + (along + local[:, 0])[:, None] * forward
            + (across + local[:, 1])[:, None] * side
            + local[:, 2:3] * up
        )
    outliers = rng.uniform([-800, -600, 100], [800, 600, 2500], (2000, 3))
    return np.concatenate([table, *clutter, outliers])


def glass_on_table(
    up: NDArray[np.float64],
    table_mm: float,
    base_front_px: tuple[float, float],
    radius_mm: float,
    height_mm: float,
    tilt_deg: float,
) -> SyntheticGlass:
    ray = np.linalg.solve(INTRINSICS.matrix(), np.array([base_front_px[0], base_front_px[1], 1.0]))
    range_mm = float(np.linalg.norm(ray * -table_mm / (up @ ray)))
    return make_glass(up, base_front_px, range_mm, radius_mm, height_mm, tilt_deg)
