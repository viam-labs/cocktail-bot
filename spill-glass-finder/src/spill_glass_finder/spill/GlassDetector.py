# Vendored from SPILL, glassloc/GlassDetector.py at commit 7fe7282d29730c6026d3dd682c6d926ee3f52735
# https://github.com/Louadria/SPILL
#
# MIT License
#
# Copyright (c) 2025 Louis Adriaens
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.
#
# Changes: YOLO is replaced by an injected detector with the same call/result shape (the robot's vision service),
# CPU instead of CUDA, airo imports removed.
import numpy as np
from typing import List
import torch
from torchvision.transforms.functional import to_tensor

from keypoint_detection.utils.heatmap import get_keypoints_from_heatmap_batch_maxpool
from keypoint_detection.utils.load_checkpoints import get_model_from_wandb_checkpoint, load_from_checkpoint

class GlassDetector:
    def __init__(self, classes: List[str], keypoint_detector: str, detector) -> None:
        self._classes = classes
        self._detector = detector
        self._keypoint_detector = load_from_checkpoint(keypoint_detector)
        self._keypoint_detector.eval()
        self._keypoint_detector.cpu()

    def get_glass_bounding_boxes(self, image: np.ndarray) -> np.ndarray | None:
        """Run object detection, detecting glasses ("cup" and "wine glass") and return the bounding boxes.

        Args:
            image: The image to detect on.

        Returns:
            An array of shape (N, 4) containing the xyxy coordinates in the original image shape of the bounding box(es), or None if there are no detections."""
        results = self._detector(image)
        result = results[0]
        bounding_boxes = []
        for i, box in enumerate(result.boxes):
            if self._detector.names[int(box.cls)] in self._classes:
                box = box.xyxyn.cpu().numpy()
                box[0, 0] *= image.shape[1]
                box[0, 1] *= image.shape[0]
                box[0, 2] *= image.shape[1]
                box[0, 3] *= image.shape[0]
                bounding_boxes.append(box)
        return np.stack(bounding_boxes) if len(bounding_boxes) > 0 else None

    def keypoint_detector_local_inference(self, image: np.ndarray, device="cpu"):
        """inference on a single image as if you would load the image from disk or get it from a camera.
        Returns a list of the extracted keypoints for each channel.


        """
        # assert model is in eval mode! (important for batch norm layers)
        assert self._keypoint_detector.training == False, "model should be in eval mode for inference"

        # convert image to tensor with correct shape (channels, height, width) and convert to floats in range [0,1]
        # add batch dimension
        # and move to device
        image = to_tensor(image).unsqueeze(0).to(device)

        # pass through model
        with torch.no_grad():
            heatmaps = self._keypoint_detector(image).squeeze(0)

        # extract keypoints from heatmaps
        predicted_keypoints = get_keypoints_from_heatmap_batch_maxpool(heatmaps.unsqueeze(0))[0]

        return predicted_keypoints
