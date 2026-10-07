from dataclasses import dataclass
from typing import Any, Mapping

import numpy as np
from numpy.typing import NDArray

from .pose import pose_config_to_matrix_m

DEFAULT_LABELS: tuple[str, ...] = ("cup", "vase", "wine glass")


@dataclass(frozen=True)
class GlassFinderConfig:
    camera_name: str
    detector_name: str
    checkpoint_path: str
    world_from_camera_m: NDArray[np.float64]
    labels: tuple[str, ...] = DEFAULT_LABELS
    min_confidence: float = 0.5
    surface_points: int = 2000
    table_crop_height_min_m: float = 0.3
    table_crop_height_max_m: float = 1.3


def _string(attrs: Mapping[str, Any], key: str) -> str:
    value = attrs.get(key)
    if not isinstance(value, str) or not value:
        raise ValueError(f"attribute {key!r} is required and must be a non-empty string")
    return value


def _number(attrs: Mapping[str, Any], key: str, default: float, minimum: float, maximum: float) -> float:
    value = attrs.get(key, default)
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise ValueError(f"attribute {key!r} must be a number, got {value!r}")
    if not minimum <= value <= maximum:
        raise ValueError(f"attribute {key!r} must be in [{minimum}, {maximum}], got {value}")
    return float(value)


def parse_config(attrs: Mapping[str, Any]) -> GlassFinderConfig:
    """Parse the resource's attributes (as a plain dict, e.g. from viam.utils.struct_to_dict)."""
    labels = attrs.get("labels", list(DEFAULT_LABELS))
    if not isinstance(labels, list) or not labels or not all(isinstance(label, str) and label for label in labels):
        raise ValueError("attribute 'labels' must be a non-empty list of strings")
    surface_points = _number(attrs, "surface_points", 2000, 1, 1_000_000)
    if surface_points != int(surface_points):
        raise ValueError(f"attribute 'surface_points' must be an integer, got {surface_points}")
    pose = attrs.get("camera_pose_in_world")
    if not isinstance(pose, Mapping):
        raise ValueError("attribute 'camera_pose_in_world' is required: {translation: {x, y, z}, orientation: {...}}")
    try:
        world_from_camera = pose_config_to_matrix_m(pose)
    except ValueError as err:
        raise ValueError(f"attribute 'camera_pose_in_world': {err}") from err
    crop_min = _number(attrs, "table_crop_height_min_m", 0.3, -100.0, 100.0)
    crop_max = _number(attrs, "table_crop_height_max_m", 1.3, -100.0, 100.0)
    if crop_min >= crop_max:
        raise ValueError(f"table_crop_height_min_m ({crop_min}) must be below table_crop_height_max_m ({crop_max})")
    return GlassFinderConfig(
        camera_name=_string(attrs, "camera_name"),
        detector_name=_string(attrs, "detector_name"),
        checkpoint_path=_string(attrs, "checkpoint_path"),
        world_from_camera_m=world_from_camera,
        labels=tuple(labels),
        min_confidence=_number(attrs, "min_confidence", 0.5, 0.0, 1.0),
        surface_points=int(surface_points),
        table_crop_height_min_m=crop_min,
        table_crop_height_max_m=crop_max,
    )
