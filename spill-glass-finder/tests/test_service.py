import asyncio
import re
from typing import Any

import numpy as np
import pytest
from heatmap_stub import StubHeatmapModel
from numpy.typing import NDArray
from PIL import Image
from synthetic import INTRINSICS, SyntheticGlass, camera_up, glass_on_table, tabletop_cloud
from viam.components.camera import Camera
from viam.media.utils.pil import pil_to_viam_image
from viam.media.video import CameraMimeType, NamedImage, ViamImage
from viam.proto.app.robot import ComponentConfig
from viam.proto.common import ResponseMetadata
from viam.proto.component.camera import IntrinsicParameters
from viam.proto.service.vision import Detection
from viam.services.vision import Vision
from viam.utils import dict_to_struct

from spill_glass_finder import service
from spill_glass_finder.keypoints import CROP_SIZE, padded_box
from spill_glass_finder.pcd import decode_pcd, encode_pcd
from spill_glass_finder.service import SpillGlassFinder
from spill_glass_finder.types import BoxDetection

TABLE_MM = 600.0
UP = camera_up(50, 3)


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


def _box(glass: SyntheticGlass, shift: int = 0) -> tuple[int, int, int, int]:
    kp = glass.keypoints
    uv = np.array([kp.bottom_front, kp.top_front, kp.top_left, kp.top_right])
    x0, y0 = np.floor(uv.min(axis=0)).astype(int) - 6 + shift
    x1, y1 = np.ceil(uv.max(axis=0)).astype(int) + 6 + shift
    return int(x0), int(y0), int(x1), int(y1)


def _crop_peaks(glass: SyntheticGlass, box: tuple[int, int, int, int]) -> list[NDArray[np.float64] | None]:
    x0, y0, x1, y1 = padded_box(BoxDetection("", 0.0, *box), INTRINSICS.width_px, INTRINSICS.height_px, 0.25)
    kp = glass.keypoints
    scale = np.array([CROP_SIZE / (x1 - x0), CROP_SIZE / (y1 - y0)])
    peaks: list[NDArray[np.float64] | None] = [
        np.round((uv - [x0, y0]) * scale) for uv in (kp.bottom_front, kp.top_front, kp.top_left, kp.top_right)
    ]
    return peaks + [None]


@pytest.fixture
def scene(monkeypatch: pytest.MonkeyPatch) -> tuple[SpillGlassFinder, list[SyntheticGlass], FakeDetector]:
    tumbler = glass_on_table(UP, TABLE_MM, (480, 520), 35.0, 110.0, 3.0)
    wine = glass_on_table(UP, TABLE_MM, (860, 470), 40.0, 180.0, 3.0)
    boxes = {"tumbler": _box(tumbler), "wine": _box(wine), "dup": _box(tumbler, shift=3)}

    def det(label: str, score: float, box: tuple[int, int, int, int]) -> Detection:
        return Detection(x_min=box[0], y_min=box[1], x_max=box[2], y_max=box[3], confidence=score, class_name=label)

    detections = [
        det("wine glass", 0.85, boxes["wine"]),
        det("person", 0.99, (0, 0, 200, 300)),
        det("cup", 0.95, boxes["tumbler"]),
        det("cup", 0.30, (1000, 100, 1100, 200)),
        det("cup", 0.70, boxes["dup"]),
    ]
    stub = StubHeatmapModel(
        [_crop_peaks(tumbler, boxes["tumbler"]), _crop_peaks(wine, boxes["wine"]), _crop_peaks(tumbler, boxes["dup"])]
    )
    monkeypatch.setattr(service, "_load_model", lambda path: (stub, 5))

    image = pil_to_viam_image(
        Image.new("RGB", (INTRINSICS.width_px, INTRINSICS.height_px), (90, 80, 70)), CameraMimeType.PNG
    )
    camera = FakeCamera(image, tabletop_cloud(UP, TABLE_MM, np.random.default_rng(7)))
    detector = FakeDetector(detections)
    attrs = {"camera_name": "cam", "detector_name": "yolov8", "checkpoint_path": "unused.ckpt", "surface_points": 1000}
    finder = SpillGlassFinder.new(
        ComponentConfig(name="glasses", attributes=dict_to_struct(attrs)),
        {Camera.get_resource_name("cam"): camera, Vision.get_resource_name("yolov8"): detector},  # type: ignore[dict-item]
    )
    return finder, [tumbler, wine], detector


def test_do_command_find_glasses(scene: tuple[SpillGlassFinder, list[SyntheticGlass], FakeDetector]) -> None:
    finder, truth, _ = scene
    result = asyncio.run(finder.do_command({"find_glasses": {}}))
    dict_to_struct(result)  # must survive the proto round trip

    glasses = result["glasses"]
    assert [g["label"] for g in glasses] == ["cup", "wine glass"]
    for got, want in zip(glasses, truth):
        assert got["frame"] == "cam"
        assert np.linalg.norm(np.array(got["rim_center_mm"]) - want.rim_center_mm) < 5.0
        assert np.linalg.norm(np.array(got["base_front_mm"]) - want.base_front_mm) < 5.0
        assert got["radius_mm"] == pytest.approx(want.radius_mm, abs=2.0)
        assert got["height_mm"] == pytest.approx(want.height_mm, abs=4.0)
        assert got["tilt_deg"] == 3.0
        assert got["reprojection_rmse_px"] < 1.5
        assert set(got["keypoints_px"]) == {"bottom_front", "top_front", "top_left", "top_right"}
    table = result["table"]
    assert np.degrees(np.arccos(np.dot(table["normal"], UP))) < 1.0
    assert table["d_mm"] == pytest.approx(TABLE_MM, abs=2.0)


def test_detections_include_keypoint_markers(
    scene: tuple[SpillGlassFinder, list[SyntheticGlass], FakeDetector],
) -> None:
    finder, _, _ = scene
    detections = asyncio.run(finder.get_detections_from_camera("cam"))
    labels = [d.class_name for d in detections]
    assert len(detections) == 10
    assert re.fullmatch(r"cup r=3[4-6]mm h=1(0[7-9]|1[0-3])mm", labels[0]), labels[0]
    assert re.fullmatch(r"wine glass r=(39|40|41)mm h=1(7[6-9]|8[0-4])mm", labels[5]), labels[5]
    assert labels[1:5] == ["kp-bottom_front", "kp-top_front", "kp-top_left", "kp-top_right"]
    assert all(d.x_max - d.x_min == 8 for d in detections if d.class_name.startswith("kp-"))


def test_object_point_clouds(scene: tuple[SpillGlassFinder, list[SyntheticGlass], FakeDetector]) -> None:
    finder, truth, _ = scene
    objects = asyncio.run(finder.get_object_point_clouds("cam", extra={"debug": True}))
    assert len(objects) == 4
    for obj, want in zip(objects[:2], truth):
        points = decode_pcd(obj.point_cloud)
        assert points.shape == (1000, 3)
        geometry = obj.geometries.geometries[0]
        assert obj.geometries.reference_frame == "cam"
        center = np.array([geometry.center.x, geometry.center.y, geometry.center.z])
        assert np.linalg.norm(center - want.rim_center_mm) < 5.0
        rim_distance = np.linalg.norm(points - center, axis=1)
        assert rim_distance.max() < np.hypot(want.radius_mm, want.height_mm) + 10.0
    raw = objects[2]
    assert raw.geometries.geometries[0].label.endswith("-raw")
    assert decode_pcd(raw.point_cloud).shape[0] > 100


def test_capture_all(scene: tuple[SpillGlassFinder, list[SyntheticGlass], FakeDetector]) -> None:
    finder, _, detector = scene
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


def test_properties_and_errors(scene: tuple[SpillGlassFinder, list[SyntheticGlass], FakeDetector]) -> None:
    finder, _, _ = scene
    props = asyncio.run(finder.get_properties())
    assert props.detections_supported and props.object_point_clouds_supported and not props.classifications_supported
    with pytest.raises(ValueError, match="configured for camera"):
        asyncio.run(finder.get_detections_from_camera("other"))
    with pytest.raises(NotImplementedError):
        asyncio.run(finder.get_classifications_from_camera("cam", 1))
    with pytest.raises(ValueError, match="unknown command"):
        asyncio.run(finder.do_command({"dance": {}}))
