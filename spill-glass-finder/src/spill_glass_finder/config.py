from dataclasses import dataclass
from typing import Any, Mapping

DEFAULT_LABELS: tuple[str, ...] = ("wine glass", "cup", "vase")
CHANNEL_ORDERS = ("rgb", "bgr")


@dataclass(frozen=True)
class GlassFinderConfig:
    camera_name: str
    detector_name: str
    checkpoint_path: str
    labels: tuple[str, ...] = DEFAULT_LABELS
    min_confidence: float = 0.5
    crop_padding: float = 0.25
    table_max_depth_mm: float = 2000.0
    table_ransac_threshold_mm: float = 5.0
    table_offset_mm: float = 0.0
    surface_points: int = 2000
    channel_order: str = "rgb"
    tilt_deg: float = 3.0


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
    channel_order = attrs.get("channel_order", "rgb")
    if channel_order not in CHANNEL_ORDERS:
        raise ValueError(f"attribute 'channel_order' must be one of {CHANNEL_ORDERS}, got {channel_order!r}")
    surface_points = _number(attrs, "surface_points", 2000, 1, 1_000_000)
    if surface_points != int(surface_points):
        raise ValueError(f"attribute 'surface_points' must be an integer, got {surface_points}")
    return GlassFinderConfig(
        camera_name=_string(attrs, "camera_name"),
        detector_name=_string(attrs, "detector_name"),
        checkpoint_path=_string(attrs, "checkpoint_path"),
        labels=tuple(labels),
        min_confidence=_number(attrs, "min_confidence", 0.5, 0.0, 1.0),
        crop_padding=_number(attrs, "crop_padding", 0.25, 0.0, 2.0),
        table_max_depth_mm=_number(attrs, "table_max_depth_mm", 2000.0, 1.0, 1e6),
        table_ransac_threshold_mm=_number(attrs, "table_ransac_threshold_mm", 5.0, 0.01, 1000.0),
        table_offset_mm=_number(attrs, "table_offset_mm", 0.0, -1000.0, 1000.0),
        surface_points=int(surface_points),
        channel_order=channel_order,
        tilt_deg=_number(attrs, "tilt_deg", 3.0, 0.0, 10.0),
    )
