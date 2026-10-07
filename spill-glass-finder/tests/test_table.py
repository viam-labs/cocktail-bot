import numpy as np
import pytest
from numpy.typing import NDArray
from synthetic import camera_up, tabletop_cloud

from spill_glass_finder.table import TableNotFoundError, fit_table_plane


def _angle_deg(a: NDArray[np.float64], b: NDArray[np.float64]) -> float:
    return float(np.rad2deg(np.arccos(np.clip(a @ b, -1.0, 1.0))))


@pytest.mark.parametrize("pitch_deg, roll_deg, height_mm", [(30, 0, 400), (50, 8, 600), (70, -5, 800)])
def test_recovers_table_plane(pitch_deg: float, roll_deg: float, height_mm: float) -> None:
    up = camera_up(pitch_deg, roll_deg)
    cloud = tabletop_cloud(up, height_mm, np.random.default_rng(1))
    plane = fit_table_plane(cloud, max_depth_mm=2000.0, ransac_threshold_mm=5.0)
    assert _angle_deg(plane.normal, up) < 1.0
    assert abs(plane.d_mm - height_mm) < 2.0
    assert 0.5 < plane.inlier_ratio <= 1.0


def test_offset_moves_plane_along_normal() -> None:
    up = camera_up(50)
    cloud = tabletop_cloud(up, 600.0, np.random.default_rng(2))
    base = fit_table_plane(cloud, 2000.0, 5.0)
    lowered = fit_table_plane(cloud, 2000.0, 5.0, offset_mm=-5.0)
    np.testing.assert_allclose(lowered.normal, base.normal)
    assert lowered.d_mm == pytest.approx(base.d_mm + 5.0)


def test_rejects_plane_parallel_to_optical_axis() -> None:
    rng = np.random.default_rng(3)
    wall = np.column_stack([np.full(5000, 300.0), rng.uniform(-300, 300, 5000), rng.uniform(200, 1500, 5000)])
    with pytest.raises(TableNotFoundError, match="optical axis"):
        fit_table_plane(wall, 2000.0, 5.0)


def test_rejects_too_few_points() -> None:
    with pytest.raises(TableNotFoundError, match="valid cloud points"):
        fit_table_plane(np.full((100, 3), np.nan), 2000.0, 5.0)


def test_rejects_unstructured_cloud() -> None:
    cloud = np.random.default_rng(4).uniform([-1000, -1000, 100], [1000, 1000, 1900], (20000, 3))
    with pytest.raises(TableNotFoundError, match="inliers"):
        fit_table_plane(cloud, 2000.0, 1.0)
