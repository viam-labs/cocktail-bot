"""Loads the original SPILL glassloc.py from a local checkout with its airo/open3d/visualizer imports stubbed."""

import importlib.util
import os
import sys
import types
from pathlib import Path
from typing import Any

import numpy as np

SPILL_GLASSLOC = Path(os.environ.get("SPILL_REPO", "/Users/robin/bov/SPILL")) / "glassloc" / "glassloc.py"
SPILL_GLASS_DETECTOR = SPILL_GLASSLOC.with_name("GlassDetector.py")


class _NumpyO3DPointCloud:
    """Mimics the o3d.t PointCloud calls localize_table makes, in numpy."""

    def __init__(self, points: np.ndarray) -> None:
        self.points = np.asarray(points, dtype=np.float64)

    def transform(self, x: np.ndarray) -> "_NumpyO3DPointCloud":
        return _NumpyO3DPointCloud(self.points @ x[:3, :3].T + x[:3, 3])

    def translate(self, v: Any) -> "_NumpyO3DPointCloud":
        return _NumpyO3DPointCloud(self.points + np.asarray(v, dtype=np.float64))

    def to_legacy(self) -> "_NumpyO3DPointCloud":
        return self


class StubGlassDetector:
    """Stands in for airo_barista's GlassDetector; tests set boxes and the per-call keypoint outputs."""

    boxes: np.ndarray | None = None
    keypoint_outputs: list[list[list[list[int]]]] = []

    def __init__(self, classes: list[str], keypoint_detector: str) -> None:
        self.calls = 0

    def get_glass_bounding_boxes(self, image: np.ndarray) -> np.ndarray | None:
        return StubGlassDetector.boxes

    def keypoint_detector_local_inference(self, image: np.ndarray, device: str = "cuda") -> list[list[list[int]]]:
        out = StubGlassDetector.keypoint_outputs[self.calls]
        self.calls += 1
        return out


def _module(name: str, **attrs: Any) -> types.ModuleType:
    module = types.ModuleType(name)
    for key, value in attrs.items():
        setattr(module, key, value)
    return module


def _exec_with_stubs(path: Path, module_name: str, stubs: dict[str, types.ModuleType]) -> types.ModuleType:
    saved = {name: sys.modules.get(name) for name in stubs}
    sys.modules.update(stubs)
    try:
        spec = importlib.util.spec_from_file_location(module_name, path)
        assert spec is not None and spec.loader is not None
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module
    finally:
        for name, previous in saved.items():
            if previous is None:
                sys.modules.pop(name, None)
            else:
                sys.modules[name] = previous


def _airo_typing() -> types.ModuleType:
    return _module(
        "airo_typing",
        OpenCVIntImageType=np.ndarray,
        CameraIntrinsicsMatrixType=np.ndarray,
        NumpyDepthMapType=np.ndarray,
        PointCloud=object,
    )


def load_original_glass_detector() -> types.ModuleType:
    """Original GlassDetector.py; construct instances with __new__ since its __init__ needs YOLO weights and CUDA."""
    stubs = {
        "ultralytics": _module("ultralytics", YOLO=object),
        "airo_camera_toolkit": _module("airo_camera_toolkit"),
        "airo_camera_toolkit.utils": _module("airo_camera_toolkit.utils"),
        "airo_camera_toolkit.utils.image_converter": _module(
            "airo_camera_toolkit.utils.image_converter", ImageConverter=object
        ),
        "airo_typing": _airo_typing(),
    }
    return _exec_with_stubs(SPILL_GLASS_DETECTOR, "spill_original_glass_detector", stubs)


def load_original_glassloc() -> types.ModuleType:
    stubs = {
        "open3d": _module("open3d"),
        "matplotlib": _module("matplotlib"),
        "matplotlib.pyplot": _module("matplotlib.pyplot"),
        "airo_camera_toolkit": _module("airo_camera_toolkit"),
        "airo_camera_toolkit.cameras": _module("airo_camera_toolkit.cameras"),
        "airo_camera_toolkit.cameras.realsense": _module("airo_camera_toolkit.cameras.realsense"),
        "airo_camera_toolkit.cameras.realsense.realsense": _module(
            "airo_camera_toolkit.cameras.realsense.realsense", Realsense=object
        ),
        "airo_camera_toolkit.point_clouds": _module("airo_camera_toolkit.point_clouds"),
        "airo_camera_toolkit.point_clouds.conversions": _module(
            "airo_camera_toolkit.point_clouds.conversions", point_cloud_to_open3d=_NumpyO3DPointCloud
        ),
        "airo_camera_toolkit.utils": _module("airo_camera_toolkit.utils"),
        "airo_camera_toolkit.utils.image_converter": _module(
            "airo_camera_toolkit.utils.image_converter", ImageConverter=object
        ),
        "airo_barista": _module("airo_barista"),
        "airo_barista.glassloc": _module("airo_barista.glassloc"),
        "airo_barista.glassloc.GlassDetector": _module(
            "airo_barista.glassloc.GlassDetector", GlassDetector=StubGlassDetector
        ),
        "airo_barista.glassloc.CameraVisualizer": _module(
            "airo_barista.glassloc.CameraVisualizer", CameraVisualizer=object
        ),
        "airo_typing": _airo_typing(),
    }
    return _exec_with_stubs(SPILL_GLASSLOC, "spill_original_glassloc", stubs)
