"""Viam frame-config poses (translation in mm, ov_degrees orientation) as 4x4 transforms in meters."""

from typing import Any, Mapping

import numpy as np
from numpy.typing import NDArray

# RDK spatialmath's threshold for treating an orientation vector as pointing along ±z.
_POLE_EPSILON = 1e-4


def _rot_z(angle: float) -> NDArray[np.float64]:
    c, s = np.cos(angle), np.sin(angle)
    return np.array([[c, -s, 0.0], [s, c, 0.0], [0.0, 0.0, 1.0]])


def _rot_y(angle: float) -> NDArray[np.float64]:
    c, s = np.cos(angle), np.sin(angle)
    return np.array([[c, 0.0, s], [0.0, 1.0, 0.0], [-s, 0.0, c]])


def orientation_vector_degrees_to_matrix(ox: float, oy: float, oz: float, theta_deg: float) -> NDArray[np.float64]:
    """Same convention as RDK's OrientationVector.Quaternion: ZYZ Euler angles (lon, lat, theta)."""
    norm = float(np.linalg.norm([ox, oy, oz]))
    if norm == 0.0:
        ox, oy, oz = 0.0, 0.0, 1.0
    else:
        ox, oy, oz = ox / norm, oy / norm, oz / norm
    lat = np.arccos(np.clip(oz, -1.0, 1.0))
    lon = np.arctan2(oy, ox) if 1.0 - abs(oz) > _POLE_EPSILON else 0.0
    return _rot_z(lon) @ _rot_y(lat) @ _rot_z(np.deg2rad(theta_deg))


def pose_config_to_matrix_m(pose: Mapping[str, Any]) -> NDArray[np.float64]:
    """{"translation": {x, y, z} mm, "orientation": {"type": "ov_degrees", "value": {x, y, z, th}}} -> 4x4 in m."""
    translation = pose.get("translation", {})
    orientation = pose.get("orientation", {"type": "ov_degrees", "value": {"x": 0, "y": 0, "z": 1, "th": 0}})
    if not isinstance(translation, Mapping) or not isinstance(orientation, Mapping):
        raise ValueError("pose needs a 'translation' object and an 'orientation' object")
    if orientation.get("type") != "ov_degrees":
        raise ValueError(f"orientation type must be 'ov_degrees', got {orientation.get('type')!r}")
    value = orientation.get("value", {})
    if not isinstance(value, Mapping):
        raise ValueError("orientation 'value' must be an object with x, y, z, th")
    numbers = [translation.get(k, 0.0) for k in ("x", "y", "z")] + [value.get(k, 0.0) for k in ("x", "y", "z", "th")]
    if not all(isinstance(n, (int, float)) and not isinstance(n, bool) for n in numbers):
        raise ValueError(f"pose values must be numbers, got {numbers}")
    x, y, z, ox, oy, oz, th = (float(n) for n in numbers)
    transform = np.eye(4)
    transform[:3, :3] = orientation_vector_degrees_to_matrix(ox, oy, oz, th)
    transform[:3, 3] = np.array([x, y, z]) / 1000.0
    return transform
