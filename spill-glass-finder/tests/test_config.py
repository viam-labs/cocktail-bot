from typing import Any

import numpy as np
import pytest
from viam.proto.app.robot import ComponentConfig
from viam.utils import dict_to_struct

from spill_glass_finder.config import DEFAULT_LABELS, parse_config
from spill_glass_finder.service import SpillGlassFinder

POSE = {
    "translation": {"x": 0, "y": 0, "z": 1250},
    "orientation": {"type": "ov_degrees", "value": {"x": 0, "y": 1, "z": -1, "th": 0}},
}
REQUIRED = {
    "camera_name": "cam",
    "detector_name": "yolov8",
    "checkpoint_path": "/opt/weights/wild_glasses.ckpt",
    "camera_pose_in_world": POSE,
}


def _component(attrs: dict[str, Any]) -> ComponentConfig:
    return ComponentConfig(name="glasses", attributes=dict_to_struct(attrs))


def test_defaults() -> None:
    config = parse_config(REQUIRED)
    assert config.labels == DEFAULT_LABELS == ("cup", "vase", "wine glass")
    assert config.min_confidence == 0.5
    assert config.surface_points == 2000
    assert config.table_crop_height_min_m == 0.3
    assert config.table_crop_height_max_m == 1.3
    np.testing.assert_allclose(config.world_from_camera_m[:3, 3], [0.0, 0.0, 1.25])
    np.testing.assert_allclose(config.world_from_camera_m[:3, 2], [0.0, np.sqrt(0.5), -np.sqrt(0.5)], atol=1e-12)


def test_validate_config_returns_dependencies() -> None:
    attrs = {
        **REQUIRED,
        "labels": ["cup"],
        "surface_points": 500,
        "table_crop_height_min_m": -0.2,
        "table_crop_height_max_m": 0.8,
    }
    assert SpillGlassFinder.validate_config(_component(attrs)) == (["cam", "yolov8"], [])


@pytest.mark.parametrize(
    "override, message",
    [
        ({"camera_name": ""}, "camera_name"),
        ({"detector_name": None}, "detector_name"),
        ({"checkpoint_path": 3}, "checkpoint_path"),
        ({"camera_pose_in_world": None}, "camera_pose_in_world"),
        ({"camera_pose_in_world": {"orientation": {"type": "quaternion", "value": {}}}}, "ov_degrees"),
        ({"camera_pose_in_world": {"translation": {"x": "far"}}}, "numbers"),
        ({"labels": []}, "labels"),
        ({"labels": "cup"}, "labels"),
        ({"min_confidence": 1.5}, "min_confidence"),
        ({"surface_points": 10.5}, "surface_points"),
        ({"table_crop_height_min_m": 1.5}, "table_crop_height_min_m"),
    ],
)
def test_invalid(override: dict[str, Any], message: str) -> None:
    attrs = {**REQUIRED, **override}
    with pytest.raises(ValueError, match=message):
        SpillGlassFinder.validate_config(_component({k: v for k, v in attrs.items() if v is not None}))
