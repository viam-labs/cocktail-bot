import itertools

import numpy as np
import pytest
from numpy.typing import NDArray
from synthetic import INTRINSICS, camera_up, make_glass

from spill_glass_finder.geometry import (
    GlassSolveError,
    deduplicate,
    glass_points,
    intersect_ray_with_plane,
    sample_surface,
    solve_glass,
)
from spill_glass_finder.types import BoxDetection, GlassEstimate, Keypoints2D, Plane

K = INTRINSICS.matrix()
DETECTION = BoxDetection("cup", 0.9, 0, 0, 1, 1)
PITCHES = (30, 50, 70)
RANGES = (300, 600, 1000)
COLUMN_OFFSETS = (-250, 0, 250)
RADII = (25, 45)
HEIGHTS = (60, 120, 200)
TILTS = (0, 4, 8)


def _spill_points(
    base_front: NDArray[np.float64], n_down: NDArray[np.float64], r: float, h: float, angle: float
) -> dict[str, NDArray[np.float64]]:
    """SPILL's glassloc.py expressions verbatim, with its downward-pointing normal."""
    backwards = base_front - n_down * np.dot(base_front, n_down)
    backwards /= np.linalg.norm(backwards)
    side = np.cross(n_down, backwards)
    side /= np.linalg.norm(side)
    return {
        "top_middle": base_front + r * backwards - h * n_down - np.tan(angle) * h * backwards,
        "top_left": base_front + r * backwards - h * n_down - np.tan(angle) * h * backwards - r * side,
        "top_right": base_front + r * backwards - h * n_down - np.tan(angle) * h * backwards + r * side,
        "top_front": base_front - h * n_down - np.tan(angle) * h * backwards,
    }


@pytest.mark.parametrize("pitch, roll, u", [(35, 0, 300.0), (55, 10, 640.0), (70, -15, 1000.0)])
def test_model_matches_spill_expressions(pitch: float, roll: float, u: float) -> None:
    up = camera_up(pitch, roll)
    d_up = 0.55
    n_down, d_down = -up, -d_up
    ray = np.linalg.solve(K, np.array([u, 500.0, 1.0]))
    spill_base_front = ray * -d_down / (n_down @ ray)
    base_front = intersect_ray_with_plane(K, np.array([u, 500.0]), up, d_up)
    np.testing.assert_allclose(base_front, spill_base_front, atol=1e-12)

    ours = glass_points(base_front, up, 0.04, 0.12, np.deg2rad(5.0))
    spill = _spill_points(spill_base_front, n_down, 0.04, 0.12, np.deg2rad(5.0))
    np.testing.assert_allclose(ours.rim_center, spill["top_middle"], atol=1e-12)
    np.testing.assert_allclose(ours.top_left, spill["top_left"], atol=1e-12)
    np.testing.assert_allclose(ours.top_right, spill["top_right"], atol=1e-12)
    np.testing.assert_allclose(ours.top_front, spill["top_front"], atol=1e-12)
    # top_left is image-left, matching the dataset labels.
    assert (K @ ours.top_left)[0] / (K @ ours.top_left)[2] < (K @ ours.top_right)[0] / (K @ ours.top_right)[2]


def _grid() -> list[tuple[int, int, int, int, int, int]]:
    return list(itertools.product(PITCHES, RANGES, COLUMN_OFFSETS, RADII, HEIGHTS, TILTS))


def test_noise_free_recovery() -> None:
    for pitch, range_mm, du, r, h, tilt in _grid():
        glass = make_glass(camera_up(pitch, 5), (642 + du, 420), range_mm, r, h, tilt)
        est = solve_glass(DETECTION, glass.keypoints, K, glass.plane, tilt_deg=tilt)
        case = (pitch, range_mm, du, r, h, tilt)
        assert np.linalg.norm(est.rim_center_mm - glass.rim_center_mm) < 1.0, case
        assert abs(est.radius_mm - r) < 1.0, case
        assert abs(est.height_mm - h) < 1.0, case
        assert est.reprojection_rmse_px < 1e-3, case


def test_one_pixel_noise_error_bounds() -> None:
    """σ = 1 px on every keypoint coordinate, tilt known. p95 bounds: rim center and height within 1% of range, radius 2 mm."""
    rng = np.random.default_rng(0)
    errors: dict[int, list[tuple[float, float, float]]] = {r: [] for r in RANGES}
    for _ in range(2):
        for pitch, range_mm, du, r, h, tilt in _grid():
            glass = make_glass(camera_up(pitch, 5), (642 + du, 420), range_mm, r, h, tilt, noise_px=1.0, rng=rng)
            est = solve_glass(DETECTION, glass.keypoints, K, glass.plane, tilt_deg=tilt)
            errors[range_mm].append(
                (
                    float(np.linalg.norm(est.rim_center_mm - glass.rim_center_mm)),
                    abs(est.radius_mm - r),
                    abs(est.height_mm - h),
                )
            )
    for range_mm, errs in errors.items():
        p95 = np.percentile(np.array(errs), 95, axis=0)
        assert p95[0] < 0.01 * range_mm, (range_mm, p95)
        assert p95[1] < 2.0, (range_mm, p95)
        assert p95[2] < 0.01 * range_mm, (range_mm, p95)


def test_wrong_tilt_assumption_shifts_rim_by_about_h_tan_delta() -> None:
    """Tilt is unobservable: a 3° error moves the rim center horizontally by roughly h·tan(3°)."""
    up = camera_up(50)
    glass = make_glass(up, (642, 420), 600, 35, 150, 0.0)
    est = solve_glass(DETECTION, glass.keypoints, K, glass.plane, tilt_deg=3.0)
    assert est.reprojection_rmse_px < 1e-3
    delta = est.rim_center_mm - glass.rim_center_mm
    horizontal = np.linalg.norm(delta - (delta @ up) * up)
    assert horizontal == pytest.approx(150 * np.tan(np.deg2rad(3.0)), rel=0.25)


def test_rejects_ray_above_horizon() -> None:
    up = camera_up(10)
    kp = Keypoints2D(*(np.array([642.0, 0.0]) for _ in range(4)))
    with pytest.raises(GlassSolveError, match="behind the camera"):
        solve_glass(DETECTION, kp, K, Plane(up, 500.0, 1.0))


def test_rejects_implausible_size() -> None:
    glass = make_glass(camera_up(50), (642, 420), 600, 35, 120, 3.0)
    kp = glass.keypoints
    huge = Keypoints2D(kp.bottom_front, kp.top_front, kp.top_left - [600.0, 0.0], kp.top_right + [600.0, 0.0])
    with pytest.raises(GlassSolveError, match="hit its bound"):
        solve_glass(DETECTION, huge, K, glass.plane)


def _estimate(score: float, center: list[float], radius: float) -> GlassEstimate:
    zero = np.zeros(2)
    return GlassEstimate(
        label="cup",
        score=score,
        bbox=(0, 0, 1, 1),
        keypoints_px=Keypoints2D(zero, zero, zero, zero),
        rim_center_mm=np.array(center),
        base_front_mm=np.array(center),
        up=np.array([0.0, -1.0, 0.0]),
        back=np.array([0.0, 0.0, 1.0]),
        radius_mm=radius,
        height_mm=100.0,
        tilt_deg=3.0,
        reprojection_rmse_px=0.0,
    )


def test_deduplicate_keeps_higher_score() -> None:
    a = _estimate(0.6, [0.0, 0.0, 500.0], 40.0)
    b = _estimate(0.9, [20.0, 0.0, 500.0], 40.0)
    c = _estimate(0.7, [200.0, 0.0, 500.0], 40.0)
    assert deduplicate([a, b, c]) == [b, c]


def test_surface_samples_lie_on_glass() -> None:
    glass = make_glass(camera_up(50), (642, 420), 600, 35, 120, 4.0)
    est = solve_glass(DETECTION, glass.keypoints, K, glass.plane, tilt_deg=4.0)
    points = sample_surface(est, 2000)
    assert points.shape == (2000, 3)
    up, back = est.up, est.back
    base_radius = est.radius_mm - est.height_mm * np.tan(np.deg2rad(4.0))
    base_center = est.base_front_mm + base_radius * back
    rel = points - base_center
    heights = rel @ up
    radial = np.linalg.norm(rel - heights[:, None] * up, axis=1)
    assert heights.min() >= -1e-6 and heights.max() <= est.height_mm + 1e-6
    np.testing.assert_allclose(radial, base_radius + heights / est.height_mm * (est.radius_mm - base_radius), atol=1e-6)
    rim = np.isclose(heights, est.height_mm)
    assert rim.sum() == 400
    np.testing.assert_allclose(points[rim].mean(axis=0), est.rim_center_mm, atol=5.0)
