from typing import Any

import pytest
from viam.proto.app.robot import ComponentConfig
from viam.utils import dict_to_struct

from spill_glass_finder.config import DEFAULT_LABELS, parse_config
from spill_glass_finder.service import SpillGlassFinder

REQUIRED = {"camera_name": "cam", "detector_name": "yolov8", "checkpoint_path": "/opt/weights/wild_glasses.ckpt"}


def _component(attrs: dict[str, Any]) -> ComponentConfig:
    return ComponentConfig(name="glasses", attributes=dict_to_struct(attrs))


def test_defaults() -> None:
    config = parse_config(REQUIRED)
    assert config.labels == DEFAULT_LABELS
    assert config.min_confidence == 0.5
    assert config.crop_padding == 0.25
    assert config.table_max_depth_mm == 2000.0
    assert config.table_ransac_threshold_mm == 5.0
    assert config.table_offset_mm == 0.0
    assert config.surface_points == 2000
    assert config.channel_order == "rgb"
    assert config.tilt_deg == 3.0


def test_validate_config_returns_dependencies() -> None:
    attrs = {**REQUIRED, "labels": ["cup"], "surface_points": 500, "channel_order": "bgr", "table_offset_mm": -5}
    assert SpillGlassFinder.validate_config(_component(attrs)) == (["cam", "yolov8"], [])


@pytest.mark.parametrize(
    "override, message",
    [
        ({"camera_name": ""}, "camera_name"),
        ({"detector_name": None}, "detector_name"),
        ({"checkpoint_path": 3}, "checkpoint_path"),
        ({"labels": []}, "labels"),
        ({"labels": "cup"}, "labels"),
        ({"min_confidence": 1.5}, "min_confidence"),
        ({"min_confidence": "high"}, "min_confidence"),
        ({"surface_points": 10.5}, "surface_points"),
        ({"channel_order": "rgba"}, "channel_order"),
        ({"tilt_deg": 12}, "tilt_deg"),
    ],
)
def test_invalid(override: dict[str, Any], message: str) -> None:
    attrs = {**REQUIRED, **override}
    with pytest.raises(ValueError, match=message):
        SpillGlassFinder.validate_config(_component({k: v for k, v in attrs.items() if v is not None}))
