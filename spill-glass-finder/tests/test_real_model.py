import json
import os
from pathlib import Path

import cv2
import numpy as np
import pytest
from PIL import Image
from spill_reference import SPILL_GLASS_DETECTOR, load_original_glass_detector

from spill_glass_finder.detections import FrameDetections
from spill_glass_finder.spill.GlassDetector import GlassDetector
from spill_glass_finder.types import KEYPOINT_NAMES

MODULE_ROOT = Path(__file__).resolve().parents[1]
CHECKPOINT = Path(os.environ.get("SPILL_CHECKPOINT", MODULE_ROOT / "weights" / "wild_glasses.ckpt"))
FIXTURES = Path(__file__).parent / "fixtures" / "glasses_in_the_wild"
OUTPUT = Path(__file__).parent / "output"
# The fixtures are already SPILL crops: with SPILL's padding this box pads back out to (1, 1, 255, 255).
BOX = (43, 43, 213, 213)
MEDIAN_ERROR_PX = 8.0

pytestmark = pytest.mark.skipif(not CHECKPOINT.is_file(), reason=f"checkpoint not found at {CHECKPOINT}")


def _spill_crop(image_bgr: np.ndarray) -> tuple[np.ndarray, tuple[int, int, int, int]]:
    """localize_glass's padding, clamping and resize."""
    x1, y1, x2, y2 = BOX
    padding_x, padding_y = int(64 * (x2 - x1) / 256), int(64 * (y2 - y1) / 256)
    x1, y1 = max(0, x1 - padding_x), max(0, y1 - padding_y)
    x2, y2 = min(x2 + padding_x, image_bgr.shape[1]), min(y2 + padding_y, image_bgr.shape[0])
    return cv2.resize(image_bgr[int(y1) : int(y2), int(x1) : int(x2)], (256, 256)), (x1, y1, x2, y2)


def test_real_model_finds_plausible_keypoints() -> None:
    detector = GlassDetector(["cup"], str(CHECKPOINT), FrameDetections())
    assert not detector._keypoint_detector.training
    assert next(detector._keypoint_detector.parameters()).device.type == "cpu"
    if SPILL_GLASS_DETECTOR.is_file():
        original_cls = load_original_glass_detector().GlassDetector
        original = original_cls.__new__(original_cls)
        original._keypoint_detector = detector._keypoint_detector
    else:
        original = None

    labels = json.loads((FIXTURES / "keypoints.json").read_text())
    annotated, errors = [], []
    for name in sorted(labels):
        image_bgr = np.ascontiguousarray(np.asarray(Image.open(FIXTURES / name).convert("RGB"))[:, :, ::-1])
        crop, (x1, y1, x2, y2) = _spill_crop(image_bgr)
        keypoints = detector.keypoint_detector_local_inference(crop)
        if original is not None:
            assert keypoints == original.keypoint_detector_local_inference(crop, device="cpu")
        canvas = image_bgr.copy()
        uv_all = []
        for channel, (color, kp_name) in enumerate(
            zip(((0, 0, 255), (0, 255, 0), (255, 0, 0), (255, 0, 255)), KEYPOINT_NAMES)
        ):
            assert keypoints[channel], (name, kp_name)
            u = keypoints[channel][0][0] / 256 * (x2 - x1) + x1
            v = keypoints[channel][0][1] / 256 * (y2 - y1) + y1
            assert x1 <= u <= x2 and y1 <= v <= y2, (name, kp_name, u, v)
            uv_all.append((u, v))
            errors.append(float(np.hypot(u - labels[name][kp_name][0], v - labels[name][kp_name][1])))
            cv2.circle(canvas, (int(u), int(v)), 5, color, -1)
            cv2.drawMarker(canvas, tuple(int(c) for c in labels[name][kp_name]), color, cv2.MARKER_CROSS, 12, 1)
        for top in uv_all[1:]:
            assert top[1] < uv_all[0][1], name
        annotated.append(canvas)

    OUTPUT.mkdir(exist_ok=True)
    cv2.imwrite(str(OUTPUT / "real_model_keypoints.png"), np.hstack(annotated))
    print("keypoint errors vs labels (px):", np.round(errors, 1))
    assert np.median(errors) < MEDIAN_ERROR_PX, errors
