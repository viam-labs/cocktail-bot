import numpy as np
import torch
from numpy.typing import NDArray

CROP_SIZE = 256


def gaussian_heatmaps(
    peaks_uv: list[NDArray[np.float64] | None], n_channels: int = 5, sigma: float = 3.0
) -> torch.Tensor:
    """(n_channels, 256, 256) heatmaps peaked at each crop-space (u, v); None leaves that channel empty."""
    v, u = torch.meshgrid(
        torch.arange(CROP_SIZE, dtype=torch.float32), torch.arange(CROP_SIZE, dtype=torch.float32), indexing="ij"
    )
    maps = torch.zeros(n_channels, CROP_SIZE, CROP_SIZE)
    for c, peak in enumerate(peaks_uv):
        if peak is not None:
            maps[c] = 0.9 * torch.exp(-((u - float(peak[0])) ** 2 + (v - float(peak[1])) ** 2) / (2 * sigma**2))
    return maps


class StubKeypointModel:
    """Stands in for the Lightning KeypointDetector: one (1, 3, 256, 256) crop per call, heatmaps in call order."""

    training = False

    def __init__(self) -> None:
        self.queue: list[torch.Tensor] = []
        self.inputs: list[torch.Tensor] = []

    def eval(self) -> "StubKeypointModel":
        return self

    def cpu(self) -> "StubKeypointModel":
        return self

    def __call__(self, x: torch.Tensor) -> torch.Tensor:
        assert x.shape == (1, 3, CROP_SIZE, CROP_SIZE) and x.dtype == torch.float32
        self.inputs.append(x)
        return self.queue.pop(0).unsqueeze(0)
