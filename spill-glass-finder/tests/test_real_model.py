import json
import os
from pathlib import Path

import cv2
import numpy as np
import pytest
from PIL import Image

from spill_glass_finder.keypoints import detect_keypoints, padded_box
from spill_glass_finder.model import load_keypoint_detector
from spill_glass_finder.types import KEYPOINT_NAMES, BoxDetection

MODULE_ROOT = Path(__file__).resolve().parents[1]
CHECKPOINT = Path(os.environ.get("SPILL_CHECKPOINT", MODULE_ROOT / "weights" / "wild_glasses.ckpt"))
FIXTURES = Path(__file__).parent / "fixtures" / "glasses_in_the_wild"
OUTPUT = Path(__file__).parent / "output"
# The fixtures are already SPILL crops; this box pads back out to (1, 1, 255, 255).
BOX = BoxDetection("cup", 0.9, 43, 43, 213, 213)
MEDIAN_ERROR_PX = 8.0

pytestmark = pytest.mark.skipif(not CHECKPOINT.is_file(), reason=f"checkpoint not found at {CHECKPOINT}")


def test_real_model_finds_plausible_keypoints() -> None:
    model, hparams = load_keypoint_detector(CHECKPOINT)
    assert len(hparams["keypoint_channel_configuration"]) == 5
    labels = json.loads((FIXTURES / "keypoints.json").read_text())
    names = sorted(labels)
    images = [np.asarray(Image.open(FIXTURES / name).convert("RGB")) for name in names]

    annotated = []
    errors = []
    for name, image in zip(names, images):
        (kp,) = detect_keypoints(
            image, [BOX], model, 0.25, "rgb", hparams["minimal_keypoint_extraction_pixel_distance"]
        )
        assert kp is not None, name
        x0, y0, x1, y1 = padded_box(BOX, image.shape[1], image.shape[0], 0.25)
        canvas = cv2.cvtColor(image, cv2.COLOR_RGB2BGR)
        for color, kp_name in zip(((0, 0, 255), (0, 255, 0), (255, 0, 0), (255, 0, 255)), KEYPOINT_NAMES):
            uv = getattr(kp, kp_name)
            assert x0 <= uv[0] <= x1 and y0 <= uv[1] <= y1, (name, kp_name, uv)
            errors.append(float(np.linalg.norm(uv - labels[name][kp_name])))
            cv2.circle(canvas, (int(uv[0]), int(uv[1])), 5, color, -1)
            cv2.drawMarker(canvas, tuple(int(c) for c in labels[name][kp_name]), color, cv2.MARKER_CROSS, 12, 1)
        for top in (kp.top_front, kp.top_left, kp.top_right):
            assert top[1] < kp.bottom_front[1], name
        annotated.append(canvas)

    OUTPUT.mkdir(exist_ok=True)
    cv2.imwrite(str(OUTPUT / "real_model_keypoints.png"), np.hstack(annotated))
    print("keypoint errors vs labels (px):", np.round(errors, 1))
    assert np.median(errors) < MEDIAN_ERROR_PX, errors
