"""Hands one frame's vision-service detections to SPILL's GlassDetector in the shape it reads from ultralytics.YOLO."""

from dataclasses import dataclass

import numpy as np
import torch
from numpy.typing import NDArray

from .types import BoxDetection


@dataclass(frozen=True)
class _Box:
    cls: torch.Tensor
    xyxyn: torch.Tensor


@dataclass(frozen=True)
class _Result:
    boxes: list[_Box]


def select_detections(
    detections: list[BoxDetection], labels: tuple[str, ...], min_confidence: float
) -> list[BoxDetection]:
    """Labels and confidence filtered, highest score first, as ultralytics orders its boxes."""
    kept = [d for d in detections if d.label in labels and d.score >= min_confidence]
    return sorted(kept, key=lambda d: d.score, reverse=True)


class FrameDetections:
    """Callable like ultralytics.YOLO: detector(image) -> [result] with result.boxes[i].cls / .xyxyn and .names."""

    def __init__(self) -> None:
        self.detections: list[BoxDetection] = []
        self.names: dict[int, str] = {}

    def set_detections(self, detections: list[BoxDetection]) -> None:
        self.detections = detections
        self.names = dict(enumerate(sorted({d.label for d in detections})))

    def __call__(self, image: NDArray[np.uint8]) -> list[_Result]:
        height, width = image.shape[:2]
        class_ids = {label: i for i, label in self.names.items()}
        return [
            _Result(
                boxes=[
                    _Box(
                        cls=torch.tensor([class_ids[d.label]], dtype=torch.float32),
                        # float32, like ultralytics' xyxyn
                        xyxyn=torch.tensor(
                            [[d.x_min / width, d.y_min / height, d.x_max / width, d.y_max / height]],
                            dtype=torch.float32,
                        ),
                    )
                    for d in self.detections
                ]
            )
        ]

    def match(self, box: NDArray[np.float64]) -> BoxDetection:
        """The detection a SPILL bounding box (pixel xyxy) came from."""
        corners = np.array([[d.x_min, d.y_min, d.x_max, d.y_max] for d in self.detections], dtype=np.float64)
        return self.detections[int(np.argmin(np.abs(corners - np.asarray(box, dtype=np.float64)).sum(axis=1)))]
