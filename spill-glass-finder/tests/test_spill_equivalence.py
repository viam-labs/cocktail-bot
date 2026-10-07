"""The vendored SPILL code must give exactly the numbers the original glassloc.py gives on the same inputs."""

from typing import Any

import numpy as np
import pytest
import torch
from heatmap_stub import StubKeypointModel
from spill_reference import SPILL_GLASSLOC, StubGlassDetector, load_original_glass_detector, load_original_glassloc
from synthetic import INTRINSICS, SyntheticGlass, make_glass, tabletop_cloud_m, world_from_camera

import spill_glass_finder.spill.GlassDetector as vendored_detector_module
from spill_glass_finder.detections import FrameDetections
from spill_glass_finder.spill.glassloc import GlassLocalizer
from spill_glass_finder.types import BoxDetection

pytestmark = pytest.mark.skipif(not SPILL_GLASSLOC.is_file(), reason=f"SPILL checkout not found at {SPILL_GLASSLOC}")

CLASSES = ["cup", "vase", "wine glass"]


class _StubModel:
    training = False

    def eval(self) -> "_StubModel":
        return self

    def cpu(self) -> "_StubModel":
        return self


@pytest.fixture
def vendored(monkeypatch: pytest.MonkeyPatch) -> tuple[GlassLocalizer, FrameDetections]:
    monkeypatch.setattr(vendored_detector_module, "load_from_checkpoint", lambda path: _StubModel())
    frame_detections = FrameDetections()
    return GlassLocalizer(INTRINSICS.matrix(), "unused.ckpt", CLASSES, frame_detections), frame_detections


def _crop_keypoints(glass: SyntheticGlass, box: np.ndarray, fluid: bool, drop: int | None) -> list[list[list[int]]]:
    """What the keypoint model would return for this glass's SPILL crop: one integer peak per channel."""
    x1, y1, x2, y2 = box
    padding_x, padding_y = int(64 * (x2 - x1) / 256), int(64 * (y2 - y1) / 256)
    x1, y1 = max(0, x1 - padding_x), max(0, y1 - padding_y)
    x2, y2 = min(x2 + padding_x, INTRINSICS.width_px), min(y2 + padding_y, INTRINSICS.height_px)
    channels = [
        [[int(round((u - x1) / (x2 - x1) * 256)), int(round((v - y1) / (y2 - y1) * 256))]]
        for u, v in glass.keypoints_px
    ]
    if drop is not None:
        channels[drop] = []
    channels.append([[120, 200], [118, 190]] if fluid else [])
    return channels


SCENES: list[dict[str, Any]] = [
    {"pitch": 45, "height": 0.5, "glasses": [((0.0, 0.7), 0.035, 0.11, 3.0)]},
    {"pitch": 35, "height": 0.4, "glasses": [((-0.15, 0.8), 0.04, 0.18, 0.0), ((0.12, 0.9), 0.03, 0.09, 6.0)]},
    {
        "pitch": 60,
        "height": 0.6,
        "glasses": [((0.05, 0.45), 0.045, 0.14, 2.0), ((0.051, 0.452), 0.045, 0.14, 2.0)],
        "expect_dropped": True,
    },
    {
        "pitch": 50,
        "height": 0.55,
        "glasses": [((0.1, 0.6), 0.03, 0.1, 3.0), ((-0.1, 0.65), 0.035, 0.12, 1.0)],
        "drop": 1,
        "expect_dropped": True,
    },
]


@pytest.mark.parametrize("scene", SCENES)
def test_localize_glass_matches_original(
    scene: dict[str, Any], vendored: tuple[GlassLocalizer, FrameDetections]
) -> None:
    localizer, frame_detections = vendored
    x_platform_camera = world_from_camera(scene["pitch"], scene["height"])
    glasses = [make_glass(x_platform_camera, *g) for g in scene["glasses"]]
    image = np.zeros((INTRINSICS.height_px, INTRINSICS.width_px, 3), dtype=np.uint8)
    frame_detections.set_detections(
        [BoxDetection("wine glass" if i % 2 else "cup", 0.9 - 0.1 * i, *g.box_px) for i, g in enumerate(glasses)]
    )
    boxes = localizer._glass_detector.get_glass_bounding_boxes(image)
    assert boxes is not None and boxes.shape == (len(glasses), 1, 4)

    keypoint_outputs = [
        _crop_keypoints(g, boxes[i][0], fluid=i == 0, drop=scene.get("drop") if i == 1 else None)
        for i, g in enumerate(glasses)
    ]
    calls = iter(keypoint_outputs)
    localizer._glass_detector.keypoint_detector_local_inference = lambda crop, device="cpu": next(calls)

    original_module = load_original_glassloc()
    StubGlassDetector.boxes = boxes
    StubGlassDetector.keypoint_outputs = keypoint_outputs
    original = original_module.GlassLocalizer(INTRINSICS.matrix(), visualize=False)

    table_height = 0.75 - 0.005
    ours = localizer.localize_glass(image, table_height, x_platform_camera, 0.0)
    theirs = original.localize_glass(image, table_height, x_platform_camera, 0.0)

    assert len(ours) == len(theirs) > 0
    for mine, upstream in zip(ours, theirs):
        np.testing.assert_array_equal(mine[0], upstream[0])
        assert mine[1] == upstream[1] and mine[2] == upstream[2]
    assert (len(ours) < len(glasses)) == scene.get("expect_dropped", False)


@pytest.mark.parametrize("pitch, height", [(35, 0.4), (50, 0.55), (65, 0.7)])
def test_localize_table_matches_original(
    pitch: float, height: float, vendored: tuple[GlassLocalizer, FrameDetections]
) -> None:
    localizer, _ = vendored
    x_platform_camera = world_from_camera(pitch, height)
    cloud = tabletop_cloud_m(x_platform_camera, np.random.default_rng(3))
    original = load_original_glassloc().GlassLocalizer(INTRINSICS.matrix(), visualize=False)

    ours = localizer.localize_table(cloud, x_platform_camera, 0.0)
    theirs = original.localize_table(cloud, x_platform_camera, 0.0)
    assert ours == theirs
    assert ours == pytest.approx(0.75 - 0.005, abs=0.003)


def test_localize_table_crop_band_is_configurable(vendored: tuple[GlassLocalizer, FrameDetections]) -> None:
    localizer, _ = vendored
    x_platform_camera = world_from_camera(50, 0.55)
    x_platform_camera[2, 3] -= 0.75
    cloud = tabletop_cloud_m(world_from_camera(50, 0.55), np.random.default_rng(4))
    assert localizer.localize_table(cloud, x_platform_camera, 0.0, -0.2, 0.8) == pytest.approx(-0.005, abs=0.003)


def _original_detector(classes: list[str]) -> Any:
    cls = load_original_glass_detector().GlassDetector
    detector = cls.__new__(cls)
    detector._classes = classes
    return detector


def test_get_glass_bounding_boxes_matches_original(vendored: tuple[GlassLocalizer, FrameDetections]) -> None:
    localizer, frame_detections = vendored
    frame_detections.set_detections(
        [
            BoxDetection("cup", 0.9, 101, 52, 233, 301),
            BoxDetection("person", 0.8, 0, 0, 640, 700),
            BoxDetection("wine glass", 0.7, 700, 33, 811, 420),
        ]
    )
    original = _original_detector(CLASSES)
    original._detector = frame_detections
    image = np.zeros((INTRINSICS.height_px, INTRINSICS.width_px, 3), dtype=np.uint8)
    ours = localizer._glass_detector.get_glass_bounding_boxes(image)
    theirs = original.get_glass_bounding_boxes(image)
    np.testing.assert_array_equal(ours, theirs)
    assert ours.shape == (2, 1, 4)
    np.testing.assert_allclose(ours[:, 0], [[101, 52, 233, 301], [700, 33, 811, 420]], atol=1e-3)


def test_keypoint_inference_matches_original(vendored: tuple[GlassLocalizer, FrameDetections]) -> None:
    localizer, _ = vendored
    model = StubKeypointModel()
    rng = np.random.default_rng(5)
    heatmaps = torch.from_numpy(rng.random((5, 256, 256), dtype=np.float32)) * 0.5
    heatmaps[3] = 0.0
    model.queue = [heatmaps, heatmaps.clone()]
    localizer._glass_detector._keypoint_detector = model
    original = _original_detector(CLASSES)
    original._keypoint_detector = model
    crop = rng.integers(0, 256, (256, 256, 3), dtype=np.uint8)

    ours = localizer._glass_detector.keypoint_detector_local_inference(crop)
    theirs = original.keypoint_detector_local_inference(crop, device="cpu")
    assert ours == theirs
    assert len(ours) == 5 and ours[3] == [] and len(ours[0]) == 20
    torch.testing.assert_close(model.inputs[0], model.inputs[1])
    torch.testing.assert_close(model.inputs[0][0, 0], torch.from_numpy(crop[:, :, 0]).float() / 255.0)
