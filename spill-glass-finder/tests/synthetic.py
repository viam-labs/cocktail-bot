"""Synthetic camera, table and glasses (meters unless a name says mm). World z is up; the table is at TABLE_Z_M."""

from dataclasses import dataclass

import numpy as np
from numpy.typing import NDArray

from spill_glass_finder.types import Intrinsics

INTRINSICS = Intrinsics(width_px=1280, height_px=720, fx=910.0, fy=905.0, cx=642.0, cy=358.0)
TABLE_Z_M = 0.75


def world_from_camera(pitch_deg: float, camera_above_table_m: float, x_m: float = 0.0) -> NDArray[np.float64]:
    """Camera at (x, 0, table + height) looking along world +y, pitched down by pitch_deg."""
    pitch = np.deg2rad(pitch_deg)
    right = np.array([1.0, 0.0, 0.0])
    forward = np.array([0.0, np.cos(pitch), -np.sin(pitch)])
    down = np.cross(forward, right)
    transform = np.eye(4)
    transform[:3, :3] = np.column_stack([right, down, forward])
    transform[:3, 3] = [x_m, 0.0, TABLE_Z_M + camera_above_table_m]
    return transform


def project(k: NDArray[np.float64], point: NDArray[np.float64]) -> NDArray[np.float64]:
    uvw = k @ point
    return uvw[:2] / uvw[2]


@dataclass(frozen=True)
class SyntheticGlass:
    keypoints_px: NDArray[np.float64]
    """(4, 2) bottom_front, top_front, top_left, top_right."""
    box_px: tuple[int, int, int, int]
    rim_center_world_m: NDArray[np.float64]


def make_glass(
    x_world_camera: NDArray[np.float64],
    base_center_world_m: tuple[float, float],
    radius_m: float,
    height_m: float,
    tilt_deg: float = 3.0,
) -> SyntheticGlass:
    """Upright glass on the table in SPILL's model: rim radius r, base radius r - h·tan(tilt)."""
    k = INTRINSICS.matrix()
    camera_from_world = np.linalg.inv(x_world_camera)
    camera_world = x_world_camera[:3, 3]
    base_radius = radius_m - height_m * np.tan(np.deg2rad(tilt_deg))
    base_center = np.array([base_center_world_m[0], base_center_world_m[1], TABLE_Z_M])
    back = base_center - camera_world
    back[2] = 0.0
    back /= np.linalg.norm(back)
    up = np.array([0.0, 0.0, 1.0])
    side = np.cross(up, back)
    rim_center = base_center + height_m * up
    points_world = [
        base_center - base_radius * back,
        rim_center - radius_m * back,
        rim_center + radius_m * side,
        rim_center - radius_m * side,
    ]
    uv = np.array([project(k, (camera_from_world @ np.append(p, 1.0))[:3]) for p in points_world])
    x0, y0 = np.floor(uv.min(axis=0)).astype(int) - 8
    x1, y1 = np.ceil(uv.max(axis=0)).astype(int) + 8
    assert x0 >= 0 and y0 >= 0 and x1 < INTRINSICS.width_px and y1 < INTRINSICS.height_px, "glass leaves the image"
    return SyntheticGlass(keypoints_px=uv, box_px=(int(x0), int(y0), int(x1), int(y1)), rim_center_world_m=rim_center)


def tabletop_cloud_m(x_world_camera: NDArray[np.float64], rng: np.random.Generator) -> NDArray[np.float64]:
    """Camera-frame cloud (m): noisy table seen through every 4th pixel, boxes on the table, and floor + outliers."""
    k = INTRINSICS.matrix()
    rotation, origin = x_world_camera[:3, :3], x_world_camera[:3, 3]
    u, v = np.meshgrid(np.arange(0, INTRINSICS.width_px, 4), np.arange(0, INTRINSICS.height_px, 4))
    rays_camera = np.linalg.solve(k, np.stack([u.ravel(), v.ravel(), np.ones(u.size)]).astype(float)).T
    rays_world = rays_camera @ rotation.T
    t = (TABLE_Z_M - origin[2]) / rays_world[:, 2]
    hit = (t > 0) & (t < 2.5)
    table_world = origin + rays_world[hit] * t[hit, None]
    table_world[:, 2] += rng.normal(0.0, 0.002, table_world.shape[0])

    boxes = [
        rng.uniform([cx - 0.04, cy - 0.04, TABLE_Z_M], [cx + 0.04, cy + 0.04, TABLE_Z_M + h], (3000, 3))
        for cx, cy, h in ((-0.25, 0.6, 0.2), (0.3, 0.8, 0.09), (0.0, 1.1, 0.3))
    ]
    floor = rng.uniform([-1.0, 0.2, 0.0], [1.0, 2.0, 0.0], (4000, 3))
    outliers = rng.uniform([-1.0, 0.0, 0.0], [1.0, 2.0, 2.0], (1500, 3))
    world = np.concatenate([table_world, *boxes, floor, outliers])
    return (world - origin) @ rotation
