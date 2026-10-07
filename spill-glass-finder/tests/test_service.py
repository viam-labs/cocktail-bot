import asyncio
import re
from collections.abc import Iterator
from typing import Any

import numpy as np
import pytest
from heatmap_stub import CROP_SIZE, StubKeypointModel, gaussian_heatmaps
from numpy.typing import NDArray
from PIL import Image
from synthetic import INTRINSICS, TABLE_Z_M, SyntheticGlass, make_glass, tabletop_cloud_m, world_from_camera
from viam.components.camera import Camera
from viam.media.utils.pil import pil_to_viam_image
from viam.media.video import CameraMimeType, NamedImage, ViamImage
from viam.proto.app.robot import ComponentConfig
from viam.proto.common import ResponseMetadata
from viam.proto.component.camera import IntrinsicParameters
from viam.proto.service.vision import Detection
from viam.services.vision import Vision
from viam.utils import dict_to_struct

import spill_glass_finder.spill.GlassDetector as vendored_detector_module
from spill_glass_finder import service
from spill_glass_finder.pcd import decode_pcd, encode_pcd
from spill_glass_finder.service import SpillGlassFinder

X_WORLD_CAMERA = world_from_camera(45, 0.5)
IMAGE_RGB = (200, 0, 10)


def _pose_config(transform: NDArray[np.float64]) -> dict[str, Any]:
    """Inverse of pose.orientation_vector_degrees_to_matrix (ZYZ lon, lat, theta)."""
    rotation = transform[:3, :3]
    ox, oy, oz = rotation[:, 2]
    lon = np.arctan2(oy, ox)
    lat = np.arccos(oz)
    c, s = np.cos(lon), np.sin(lon)
    rz_lon = np.array([[c, -s, 0], [s, c, 0], [0, 0, 1]])
    c, s = np.cos(lat), np.sin(lat)
    ry_lat = np.array([[c, 0, s], [0, 1, 0], [-s, 0, c]])
    rz_theta = ry_lat.T @ rz_lon.T @ rotation
    theta = np.degrees(np.arctan2(rz_theta[1, 0], rz_theta[0, 0]))
    x, y, z = transform[:3, 3] * 1000.0
    return {
        "translation": {"x": x, "y": y, "z": z},
        "orientation": {"type": "ov_degrees", "value": {"x": ox, "y": oy, "z": oz, "th": theta}},
    }


class FakeCamera:
    def __init__(self, image: ViamImage, cloud_mm: NDArray[np.float64]) -> None:
        self.image = image
        self.cloud_mm = cloud_mm

    async def get_images(self, **kwargs: Any) -> tuple[list[NamedImage], ResponseMetadata]:
        depth = NamedImage("depth", b"", CameraMimeType.VIAM_RAW_DEPTH)
        return [depth, NamedImage("color", self.image.data, self.image.mime_type)], ResponseMetadata()

    async def get_point_cloud(self, **kwargs: Any) -> tuple[bytes, str]:
        return encode_pcd(self.cloud_mm), CameraMimeType.PCD

    async def get_properties(self, **kwargs: Any) -> Camera.Properties:
        i = INTRINSICS
        return Camera.Properties(
            supports_pcd=True,
            intrinsic_parameters=IntrinsicParameters(
                width_px=i.width_px,
                height_px=i.height_px,
                focal_x_px=i.fx,
                focal_y_px=i.fy,
                center_x_px=i.cx,
                center_y_px=i.cy,
            ),
        )


class FakeDetector:
    def __init__(self, detections: list[Detection]) -> None:
        self.detections = detections
        self.calls = 0

    async def get_detections(self, image: ViamImage, **kwargs: Any) -> list[Detection]:
        assert image.width == INTRINSICS.width_px
        self.calls += 1
        return self.detections


def _crop_heatmaps(glass: SyntheticGlass) -> Any:
    x1, y1, x2, y2 = glass.box_px
    padding_x, padding_y = int(64 * (x2 - x1) / 256), int(64 * (y2 - y1) / 256)
    x1, y1 = max(0, x1 - padding_x), max(0, y1 - padding_y)
    x2, y2 = min(x2 + padding_x, INTRINSICS.width_px), min(y2 + padding_y, INTRINSICS.height_px)
    peaks: list[NDArray[np.float64] | None] = [
        np.array([(u - x1) / (x2 - x1) * CROP_SIZE, (v - y1) / (y2 - y1) * CROP_SIZE]) for u, v in glass.keypoints_px
    ]
    return gaussian_heatmaps(peaks + [None])


Scene = tuple[SpillGlassFinder, list[SyntheticGlass], FakeDetector, StubKeypointModel]


@pytest.fixture
def scene(monkeypatch: pytest.MonkeyPatch) -> Iterator[Scene]:
    tumbler = make_glass(X_WORLD_CAMERA, (-0.1, 0.62), 0.035, 0.11)
    wine = make_glass(X_WORLD_CAMERA, (0.12, 0.72), 0.04, 0.13)

    def det(label: str, score: float, box: tuple[int, int, int, int]) -> Detection:
        return Detection(x_min=box[0], y_min=box[1], x_max=box[2], y_max=box[3], confidence=score, class_name=label)

    detections = [
        det("wine glass", 0.85, wine.box_px),
        det("person", 0.99, (0, 0, 200, 300)),
        det("cup", 0.95, tumbler.box_px),
        det("cup", 0.30, (1000, 100, 1100, 200)),
    ]
    model = StubKeypointModel()
    model.queue = [_crop_heatmaps(tumbler), _crop_heatmaps(wine)]
    monkeypatch.setattr(vendored_detector_module, "load_from_checkpoint", lambda path: model)
    service._load_spill.cache_clear()

    image = pil_to_viam_image(
        Image.new("RGB", (INTRINSICS.width_px, INTRINSICS.height_px), IMAGE_RGB), CameraMimeType.PNG
    )
    camera = FakeCamera(image, tabletop_cloud_m(X_WORLD_CAMERA, np.random.default_rng(7)) * 1000.0)
    detector = FakeDetector(detections)
    attrs = {
        "camera_name": "cam",
        "detector_name": "yolov8",
        "checkpoint_path": "unused.ckpt",
        "surface_points": 1000,
        "camera_pose_in_world": _pose_config(X_WORLD_CAMERA),
    }
    finder = SpillGlassFinder.new(
        ComponentConfig(name="glasses", attributes=dict_to_struct(attrs)),
        {Camera.get_resource_name("cam"): camera, Vision.get_resource_name("yolov8"): detector},  # type: ignore[dict-item]
    )
    np.testing.assert_allclose(finder.config.world_from_camera_m, X_WORLD_CAMERA, atol=1e-9)
    yield finder, [tumbler, wine], detector, model
    service._load_spill.cache_clear()


def test_do_command_find_glasses(scene: Scene) -> None:
    finder, truth, _, model = scene
    result = asyncio.run(finder.do_command({"find_glasses": {}}))
    dict_to_struct(result)  # must survive the proto round trip

    # SPILL sorts by distance to the platform point (0, 1 m, table): the wine glass is nearer.
    glasses = result["glasses"]
    assert [g["label"] for g in glasses] == ["wine glass", "cup"]
    for got, want in zip(glasses, truth[::-1]):
        assert got["frame"] == "world"
        np.testing.assert_allclose(got["rim_center_mm"], want.rim_center_world_m * 1000.0, atol=15.0)
        assert set(got["keypoints_px"]) == {"bottom_front", "top_front", "top_left", "top_right"}
    assert result["table"]["frame"] == "world"
    assert result["table"]["height_mm"] == pytest.approx(TABLE_Z_M * 1000.0 - 5.0, abs=3.0)

    # SPILL's localize_glass takes BGR: channel 0 of the model input is blue.
    first = model.inputs[0][0]
    assert float(first[0].mean()) == pytest.approx(IMAGE_RGB[2] / 255.0)
    assert float(first[2].mean()) == pytest.approx(IMAGE_RGB[0] / 255.0)


def test_detections_include_keypoint_markers(scene: Scene) -> None:
    finder, _, _, _ = scene
    detections = asyncio.run(finder.get_detections_from_camera("cam"))
    labels = [d.class_name for d in detections]
    assert len(detections) == 10
    assert re.fullmatch(r"wine glass r=\d+mm h=\d+mm", labels[0]), labels[0]
    assert re.fullmatch(r"cup r=\d+mm h=\d+mm", labels[5]), labels[5]
    assert labels[1:5] == ["kp-bottom_front", "kp-top_front", "kp-top_left", "kp-top_right"]
    assert all(d.x_max - d.x_min == 8 for d in detections if d.class_name.startswith("kp-"))


def test_object_point_clouds_are_in_world(scene: Scene) -> None:
    finder, truth, _, _ = scene
    objects = asyncio.run(finder.get_object_point_clouds("cam", extra={"debug": True}))
    assert len(objects) == 4
    for obj, want in zip(objects[:2], truth[::-1]):
        assert obj.geometries.reference_frame == "world"
        points = decode_pcd(obj.point_cloud)
        assert points.shape == (1000, 3)
        center = obj.geometries.geometries[0].center
        np.testing.assert_allclose([center.x, center.y, center.z], want.rim_center_world_m * 1000.0, atol=15.0)
        assert points[:, 2].max() == pytest.approx(center.z, abs=1e-2)
    raw = decode_pcd(objects[2].point_cloud)
    assert objects[2].geometries.geometries[0].label.endswith("-raw")
    assert raw.shape[0] > 100
    assert np.median(raw[:, 2]) == pytest.approx(TABLE_Z_M * 1000.0, abs=20.0)


def test_capture_all(scene: Scene) -> None:
    finder, _, detector, _ = scene
    result = asyncio.run(
        finder.capture_all_from_camera(
            "cam", return_image=True, return_detections=True, return_object_point_clouds=True
        )
    )
    assert detector.calls == 1
    assert result.image is not None and result.image.mime_type == CameraMimeType.PNG
    assert result.detections is not None and len(result.detections) == 10
    assert result.objects is not None and len(result.objects) == 2
    assert result.classifications is None


def test_properties_and_errors(scene: Scene) -> None:
    finder, _, _, _ = scene
    props = asyncio.run(finder.get_properties())
    assert props.detections_supported and props.object_point_clouds_supported and not props.classifications_supported
    with pytest.raises(ValueError, match="configured for camera"):
        asyncio.run(finder.get_detections_from_camera("other"))
    with pytest.raises(NotImplementedError):
        asyncio.run(finder.get_classifications_from_camera("cam", 1))
    with pytest.raises(ValueError, match="unknown command"):
        asyncio.run(finder.do_command({"dance": {}}))
