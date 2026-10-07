"""Glass size and pose from four 2D keypoints, the camera intrinsics and the table plane.

Port of SPILL's GlassLocalizer.localize_glass geometry (https://github.com/Louadria/SPILL, MIT), with the
normal pointing up and the chained fsolve stages replaced by one bounded least-squares problem over
(side offset of the base, r, h).

Glass model (camera frame, meters): base front point b on the table, back = unit horizontal direction from the
camera to b, side = up × back. With rim radius r, height h and wall angle θ (SPILL's "glass angle"; the base
radius is r - h·tanθ and the rim center sits straight above the base center):
    rim_center = b + (r - h·tanθ)·back + h·up
    top_front  = rim_center - r·back
    top_left   = rim_center + r·side
    top_right  = rim_center - r·side

θ is not observable from these four keypoints: top_front/left/right all lie on the horizontal rim circle, and
scaling that circle about the camera center (c -> λc, r -> λr) keeps all three projections and stays in the
vertical plane through the camera and b, so for every λ some (h, θ) reproduces the base front exactly. SPILL's
fsolve pins θ to 3° for the same reason; here θ is an explicit input.
"""

from dataclasses import dataclass

import numpy as np
from numpy.typing import NDArray
from scipy.optimize import least_squares
from viam.logging import getLogger

from .types import BoxDetection, GlassEstimate, Keypoints2D, Plane

logger = getLogger(__name__)

RADIUS_BOUNDS_M = (0.005, 0.5)
HEIGHT_BOUNDS_M = (0.005, 0.3)
SIDE_OFFSET_BOUND_M = 0.1
SPILL_TILT_DEG = 3.0
BOUND_TOLERANCE = 1e-4
RIM_FRACTION = 0.2


class GlassSolveError(RuntimeError):
    pass


@dataclass(frozen=True)
class GlassPoints:
    base_front: NDArray[np.float64]
    rim_center: NDArray[np.float64]
    top_front: NDArray[np.float64]
    top_left: NDArray[np.float64]
    top_right: NDArray[np.float64]
    back: NDArray[np.float64]
    side: NDArray[np.float64]


def horizontal_frame(
    base_front: NDArray[np.float64], up: NDArray[np.float64]
) -> tuple[NDArray[np.float64], NDArray[np.float64]]:
    back = base_front - (base_front @ up) * up
    back = back / np.linalg.norm(back)
    return back, np.cross(up, back)


def glass_points(
    base_front: NDArray[np.float64], up: NDArray[np.float64], radius: float, height: float, tilt: float
) -> GlassPoints:
    back, side = horizontal_frame(base_front, up)
    rim_center = base_front + (radius - height * np.tan(tilt)) * back + height * up
    return GlassPoints(
        base_front=base_front,
        rim_center=rim_center,
        top_front=rim_center - radius * back,
        top_left=rim_center + radius * side,
        top_right=rim_center - radius * side,
        back=back,
        side=side,
    )


def project(k: NDArray[np.float64], point: NDArray[np.float64]) -> NDArray[np.float64]:
    uvw = k @ point
    return uvw[:2] / uvw[2]


def intersect_ray_with_plane(
    k: NDArray[np.float64], uv: NDArray[np.float64], up: NDArray[np.float64], d_m: float
) -> NDArray[np.float64]:
    ray = np.linalg.solve(k, np.array([uv[0], uv[1], 1.0]))
    denominator = up @ ray
    if np.isclose(denominator, 0.0):
        raise GlassSolveError("bottom_front ray is parallel to the table")
    t = -d_m / denominator
    if t <= 0.0:
        raise GlassSolveError("bottom_front ray hits the table behind the camera")
    return t * ray


def initial_size(
    k: NDArray[np.float64], kp: Keypoints2D, base_front: NDArray[np.float64], up: NDArray[np.float64]
) -> tuple[float, float]:
    """SPILL's closed-form guess followed by its fixed-point depth refinement."""
    back, _ = horizontal_frame(base_front, up)
    mean_focal = (k[0, 0] + k[1, 1]) / 2.0
    depth = float(np.linalg.norm(base_front))
    width_px = float(np.linalg.norm(kp.top_left - kp.top_right))
    height_px = float(np.linalg.norm(kp.top_front - kp.bottom_front))
    sin_view = np.sqrt(max(1.0 - (base_front @ up / depth) ** 2, 1e-6))
    height = height_px * depth / mean_focal / sin_view
    radius = width_px / 2.0 * depth / mean_focal

    old_height, old_radius = 0.0, 0.0
    old_height_depth, old_radius_depth = depth, depth
    for _ in range(100):
        if abs(old_height - height) <= 0.0005 and abs(old_radius - radius) <= 0.0005:
            break
        old_height, old_radius = height, radius
        height_depth = float(np.linalg.norm(base_front + height * up))
        radius_depth = float(np.linalg.norm(base_front + height * up + radius * back))
        height *= height_depth / old_height_depth
        radius *= radius_depth / old_radius_depth
        old_height_depth, old_radius_depth = height_depth, radius_depth
    return radius, height


def _model_from_params(
    params: NDArray[np.float64],
    base_front0: NDArray[np.float64],
    side0: NDArray[np.float64],
    up: NDArray[np.float64],
    tilt: float,
) -> GlassPoints:
    side_offset, radius, height = params
    return glass_points(base_front0 + side_offset * side0, up, radius, height, tilt)


def _keypoint_residuals(k: NDArray[np.float64], model: GlassPoints, kp: Keypoints2D) -> NDArray[np.float64]:
    return np.concatenate(
        [
            project(k, model.base_front) - kp.bottom_front,
            project(k, model.top_front) - kp.top_front,
            project(k, model.top_left) - kp.top_left,
            project(k, model.top_right) - kp.top_right,
        ]
    )


def solve_glass(
    detection: BoxDetection,
    kp: Keypoints2D,
    k: NDArray[np.float64],
    plane: Plane,
    tilt_deg: float = SPILL_TILT_DEG,
) -> GlassEstimate:
    tilt = float(np.deg2rad(tilt_deg))
    up = np.asarray(plane.normal, dtype=np.float64)
    d_m = plane.d_mm / 1000.0
    base_front0 = intersect_ray_with_plane(k, kp.bottom_front, up, d_m)
    _, side0 = horizontal_frame(base_front0, up)
    radius0, height0 = initial_size(k, kp, base_front0, up)

    lower = np.array([-SIDE_OFFSET_BOUND_M, RADIUS_BOUNDS_M[0], HEIGHT_BOUNDS_M[0]])
    upper = np.array([SIDE_OFFSET_BOUND_M, RADIUS_BOUNDS_M[1], HEIGHT_BOUNDS_M[1]])
    margin = (upper - lower) * 1e-3
    x0 = np.clip(np.array([0.0, radius0, height0]), lower + margin, upper - margin)

    def residuals(params: NDArray[np.float64]) -> NDArray[np.float64]:
        return _keypoint_residuals(k, _model_from_params(params, base_front0, side0, up, tilt), kp)

    result = least_squares(residuals, x0, bounds=(lower, upper), x_scale="jac", method="trf")
    if not result.success:
        raise GlassSolveError(f"least squares did not converge: {result.message}")
    side_offset, radius, height = result.x
    for name, value, (lo, hi) in (
        ("radius", radius, RADIUS_BOUNDS_M),
        ("height", height, HEIGHT_BOUNDS_M),
        ("side offset", side_offset, (-SIDE_OFFSET_BOUND_M, SIDE_OFFSET_BOUND_M)),
    ):
        if value - lo < BOUND_TOLERANCE * (hi - lo) or hi - value < BOUND_TOLERANCE * (hi - lo):
            raise GlassSolveError(f"{name} {value * 1000:.1f} mm hit its bound [{lo * 1000:.0f}, {hi * 1000:.0f}] mm")

    model = _model_from_params(result.x, base_front0, side0, up, tilt)
    errors = _keypoint_residuals(k, model, kp).reshape(4, 2)
    return GlassEstimate(
        label=detection.label,
        score=detection.score,
        bbox=(detection.x_min, detection.y_min, detection.x_max, detection.y_max),
        keypoints_px=kp,
        rim_center_mm=model.rim_center * 1000.0,
        base_front_mm=model.base_front * 1000.0,
        up=up,
        back=model.back,
        radius_mm=float(radius * 1000.0),
        height_mm=float(height * 1000.0),
        tilt_deg=float(tilt_deg),
        reprojection_rmse_px=float(np.sqrt(np.mean(np.sum(errors**2, axis=1)))),
    )


def deduplicate(glasses: list[GlassEstimate]) -> list[GlassEstimate]:
    """Drop a glass whose rim center is within the mean radius of a higher-scoring glass's rim center."""
    kept: list[GlassEstimate] = []
    for glass in sorted(glasses, key=lambda g: g.score, reverse=True):
        duplicate_of = next(
            (
                other
                for other in kept
                if np.linalg.norm(glass.rim_center_mm - other.rim_center_mm) < (glass.radius_mm + other.radius_mm) / 2.0
            ),
            None,
        )
        if duplicate_of is None:
            kept.append(glass)
        else:
            logger.info("dropping %s (score %.2f): duplicate of a higher-scoring glass", glass.label, glass.score)
    return kept


def sample_surface(glass: GlassEstimate, n_points: int, seed: int = 0) -> NDArray[np.float64]:
    """Points (mm, camera frame) on the side wall from base to rim plus the rim circle."""
    rng = np.random.default_rng(seed)
    up, back = glass.up, glass.back
    side = np.cross(up, back)
    rim_radius = glass.radius_mm
    base_radius = max(rim_radius - glass.height_mm * np.tan(np.deg2rad(glass.tilt_deg)), 0.0)
    base_center = glass.base_front_mm + base_radius * back

    n_rim = int(round(n_points * RIM_FRACTION))
    n_wall = n_points - n_rim
    t = np.concatenate([rng.uniform(0.0, 1.0, n_wall), np.ones(n_rim)])
    phi = rng.uniform(0.0, 2.0 * np.pi, n_points)
    radii = base_radius + t * (rim_radius - base_radius)
    return (
        base_center
        + (t * glass.height_mm)[:, None] * up
        + (radii * np.cos(phi))[:, None] * back
        + (radii * np.sin(phi))[:, None] * side
    )
