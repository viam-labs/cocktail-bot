import numpy as np
import torch
from heatmap_stub import StubHeatmapModel

from spill_glass_finder.keypoints import CROP_SIZE, crop_and_resize, crop_to_image, detect_keypoints, padded_box
from spill_glass_finder.types import BoxDetection


def test_padded_box_pads_quarter_and_clamps() -> None:
    det = BoxDetection("cup", 0.9, 100, 50, 300, 450)
    assert padded_box(det, 1280, 720, 0.25) == (50, 0, 350, 550)
    assert padded_box(BoxDetection("cup", 0.9, 1200, 600, 1270, 715), 1280, 720, 0.25) == (1183, 572, 1280, 720)


def test_crop_mapping_round_trip() -> None:
    image = np.zeros((720, 1280, 3), dtype=np.uint8)
    u, v = 700, 400
    image[v - 1 : v + 2, u - 1 : u + 2] = 255
    box = padded_box(BoxDetection("cup", 0.9, 640, 300, 840, 560), 1280, 720, 0.25)
    crop = crop_and_resize(image, box)
    assert crop.shape == (CROP_SIZE, CROP_SIZE, 3)
    peak_v, peak_u = np.unravel_index(np.argmax(crop[..., 0]), crop.shape[:2])
    mapped = crop_to_image(np.array([peak_u, peak_v], dtype=float), box)
    scale = np.array([box[2] - box[0], box[3] - box[1]]) / CROP_SIZE
    assert np.all(np.abs(mapped - [u, v]) <= scale + 0.5)


def test_detect_keypoints_maps_peaks_and_skips_incomplete() -> None:
    image = np.zeros((720, 1280, 3), dtype=np.uint8)
    image[..., 0] = 200
    detections = [BoxDetection("cup", 0.9, 100, 100, 300, 400), BoxDetection("wine glass", 0.8, 600, 200, 700, 500)]
    crop_peaks = [np.array([128.0, 220.0]), np.array([128.0, 40.0]), np.array([40.0, 60.0]), np.array([216.0, 60.0])]
    stub = StubHeatmapModel([[*crop_peaks, None], [*crop_peaks[:3], None, None]])

    result = detect_keypoints(image, detections, stub, 0.25, "rgb", 5)

    assert result[1] is None
    kp = result[0]
    assert kp is not None
    box = padded_box(detections[0], 1280, 720, 0.25)
    np.testing.assert_allclose(kp.bottom_front, crop_to_image(crop_peaks[0], box))
    np.testing.assert_allclose(kp.top_right, crop_to_image(crop_peaks[3], box))
    x = stub.inputs[0]
    assert x.shape == (2, 3, CROP_SIZE, CROP_SIZE) and x.dtype == torch.float32
    assert torch.allclose(x[:, 0], torch.full_like(x[:, 0], 200 / 255)) and float(x[:, 2].max()) == 0.0


def test_bgr_flips_channels() -> None:
    image = np.zeros((100, 100, 3), dtype=np.uint8)
    image[..., 0] = 255
    stub = StubHeatmapModel([[None] * 5])
    detect_keypoints(image, [BoxDetection("cup", 0.9, 10, 10, 90, 90)], stub, 0.25, "bgr", 5)
    assert float(stub.inputs[0][:, 2].min()) == 1.0 and float(stub.inputs[0][:, 0].max()) == 0.0
