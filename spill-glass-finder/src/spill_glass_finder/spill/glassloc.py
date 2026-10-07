# Vendored from SPILL, glassloc/glassloc.py at commit 7fe7282d29730c6026d3dd682c6d926ee3f52735
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
# Changes: CPU only; airo/open3d/matplotlib imports and the CameraVisualizer removed; print -> logger.debug;
# fluid level dropped; point_cloud_to_open3d replaced with numpy; the checkpoint path, classes, detector and the
# table crop band are parameters; each glass also carries (glass_angle, bounding box, image keypoints).
from typing import List

import cv2
import numpy as np
from numpy import ndarray
from scipy.optimize import fsolve
from viam.logging import getLogger

from .GlassDetector import GlassDetector

logger = getLogger(__name__)


class GlassLocalizer:
    def __init__(self, camera_intrinsics: np.ndarray, checkpoint_name: str, classes: List[str], detector):
        self.camera_intrinsics = camera_intrinsics
        # checkpoint_name = "checkpoints/auto_annotated_s10_longrun.ckpt"
        # checkpoint_name = "checkpoints/clearpose_glasses.ckpt"
        # checkpoint_name = "checkpoints/clearpose_extra_liquids.ckpt"
        self._glass_detector = GlassDetector(classes=classes, keypoint_detector=checkpoint_name, detector=detector)

    def localize_table(self, point_cloud: np.ndarray, X_Platform_Camera, platform_height, crop_height_min=0.3, crop_height_max=1.3) -> float:
        """Localize the table in the point cloud, assuming z is the vertical axis.

        Args:
            point_cloud: The point cloud to localize the table in. The point cloud should be in the camera frame.
            X_Platform_Camera: The transformation from the camera frame to the Platform frame.
            platform_height: The height of the Platform frame relative to the ground.
        Returns:
            The height of the table in the Platform frame."""
        # print(f"Localizing table in point cloud with {len(point_cloud.points)} points.")
        full_pcd = point_cloud @ X_Platform_Camera[:3, :3].T + X_Platform_Camera[:3, 3]
        full_pcd = full_pcd + np.array([0, 0, platform_height])
        # Find the plane
        heights = full_pcd[:, 2]
        # filter out points between -0.05 and 1.5 m
        heights = heights[(heights > crop_height_min) & (heights < crop_height_max)]
        # count bins
        hist, bin_edges = np.histogram(heights, bins=500, range=(np.min(heights), np.max(heights))) # each bin is 0.002 m = 2 mm
        # print(f"hist: {hist}")
        # # make a plot
        # plt.hist(heights, bins=500, range=(np.min(heights), np.max(heights)))
        # plt.show()
        # find the maximum bin
        max_bin = np.argmax(hist)
        max_bin_height = (bin_edges[max_bin] + bin_edges[max_bin + 1]) / 2 - 0.005  # subtract 5 mm because of experimental offset
        # Get the plane equation
        [a, b, c, d] = [0, 0, 1, -max_bin_height]
        n = np.array([a, b, c]) * (1 / np.linalg.norm([a, b, c]))
        logger.debug(f"Plane equation: {a:.6f}x + {b:.6f}y + {c:.6f}z + {d:.6f} = 0 -> normal: {n}")
        return -d


    def localize_glass(self, image: np.ndarray, table_height: float, X_Platform_Camera, platform_height):
        """Localize a glass in the image.

        Args:
            image: The image to localize the glass in.
            table_height: The height of the table in the Platform frame.
            X_Platform_Camera: The transformation from the camera frame to the Platform frame.
            platform_height: The height of the Platform frame relative to the ground.

        Returns:
            The 3D location of all glasses in the image, or an empty list if no glasses were found."""
        # Get the bounding boxes of the glasses
        glass_bounding_boxes = self._glass_detector.get_glass_bounding_boxes(image)
        glasses = []
        meshes = []
        if glass_bounding_boxes is not None:
            # Get the plane equation
            [a, b, c, d] = np.transpose(X_Platform_Camera) @ np.array([0.0, 0.0, -1.0, (table_height - platform_height)])
            n = np.array([a, b, c])
            logger.debug(f"Plane equation: {a:.6f}x + {b:.6f}y + {c:.6f}z + {d:.6f} = 0 -> normal: {n}")

            # to check if glasses are upright
            n_proj = self.camera_intrinsics @ n
            n_proj = np.array([n_proj[0] / n_proj[2], n_proj[1] / n_proj[2]])
            n_proj /= np.linalg.norm(n_proj)

            for bb in glass_bounding_boxes:
                # 1. Keypoint Detection
                # crop the image to the bounding box and resize it to 256x256
                width_o, height_o = image.shape[1], image.shape[0]
                x1, y1, x2, y2 = bb[0]
                padding_x = int(64 * (x2 - x1) / 256)
                padding_y = int(64 * (y2 - y1) / 256)
                logger.debug(f" > Padding: {padding_x}, {padding_y}")
                # padding = int(48 * ((x2 - x1) + (y2 - y1)) / (256 + 256))
                x1, y1, x2, y2 = max(0, x1 - padding_x), max(0, y1 - padding_y), min(x2 + padding_x,image.shape[1]), min(y2 + padding_y, image.shape[0])
                cropped_image = image[int(y1):int(y2), int(x1):int(x2)]
                resized_cropped_image = cv2.resize(cropped_image, (256, 256))

                # keypoint detection
                keypoints = self._glass_detector.keypoint_detector_local_inference(resized_cropped_image)
                logger.debug(f"Keypoints: {keypoints}")

                # inverse transform the keypoints to the original image
                keypoints_o = []
                for i in range(4):
                    keypoint = keypoints[i]
                    if len(keypoint) > 0:
                        keypoints_o.append(keypoint[0])
                logger.debug(f"Keypoints_o: {keypoints_o}")
                keypoints_o = np.array(keypoints_o)
                keypoints_o[:, 0] = keypoints_o[:, 0] / 256 * (x2 - x1) + x1
                keypoints_o[:, 1] = keypoints_o[:, 1] / 256 * (y2 - y1) + y1
                if len(keypoints_o) < 4:
                    logger.debug(f"Couldn't find all keypoints.")
                    continue

                bottom_front_2d = keypoints_o[0]
                top_left_2d = keypoints_o[2]
                top_right_2d = keypoints_o[3]
                top_front_2d = keypoints_o[1]
                top_middle_2d = (top_left_2d + top_right_2d) / 2
                front_center_2d = (bottom_front_2d + top_front_2d) / 2
                width_2d = np.linalg.norm(top_left_2d - top_right_2d)
                logger.debug(f"Width: {width_2d}")
                height_2d = np.linalg.norm(top_front_2d - bottom_front_2d)
                logger.debug(f"Height: {height_2d}")
                # check if glass is upright
                top_bottom_vector_2d = (top_front_2d - bottom_front_2d) / np.linalg.norm(top_front_2d - bottom_front_2d)
                logger.debug(f"top_bottom_vector_2d . n_proj: {np.dot(top_bottom_vector_2d / np.linalg.norm(top_bottom_vector_2d), n_proj)}")

                # 4. Calculate radius and height of the glass
                # Find intersection point of the plane and bottommost point (2D)
                bottom_front_ray = ( np.linalg.inv(self.camera_intrinsics) @ np.array([bottom_front_2d[0], bottom_front_2d[1], 1.0]) ) #TODO: transpose?
                logger.debug(f"bottom_front_ray: {bottom_front_ray}")
                denominator = n @ bottom_front_ray
                if np.isclose(denominator, 0):
                    logger.debug("No intersection point found.")
                    continue
                bottom_front_3d = bottom_front_ray * -d / denominator
                backwards_vector_3d = bottom_front_3d - n * np.dot(bottom_front_3d, n)
                backwards_vector_3d /= np.linalg.norm(backwards_vector_3d)
                side_vector_3d = np.cross(n, backwards_vector_3d)
                side_vector_3d /= np.linalg.norm(side_vector_3d)
                logger.debug(f"Glass at bottom_front_3d: {bottom_front_3d} -> {X_Platform_Camera @ np.append(bottom_front_3d, 1.0)}, with axes: {backwards_vector_3d}, {side_vector_3d}")

                # get rough radius and height estimates
                fx, fy = self.camera_intrinsics[0, 0], self.camera_intrinsics[1, 1]
                glass_depth = np.linalg.norm(bottom_front_3d)
                height_3d = height_2d * glass_depth * 2 / (fx + fy) / np.sqrt(1 - (np.dot(bottom_front_3d, n)/np.linalg.norm(bottom_front_3d)) ** 2)  # initial guess
                logger.debug(f"Initial height: {height_3d:.5f}")
                radius_3d = width_2d / 2 * glass_depth * 2 / (fx + fy)  # initial guess
                logger.debug(f"Initial radius: {radius_3d:.5f}=={radius_3d*2:.5f}d")

                old_height_3d = 0.0
                old_radius_3d = 0.0
                old_height_depth = glass_depth
                old_radius_depth = glass_depth
                while np.abs(old_height_3d - height_3d) > 0.0005 or np.abs(old_radius_3d - radius_3d) > 0.0005: # 0.5 mm
                    old_height_3d = height_3d
                    old_radius_3d = radius_3d
                    height_depth = np.linalg.norm(bottom_front_3d - height_3d * n)
                    # height_depth = np.linalg.norm(bottom_front_3d)
                    radius_depth = np.linalg.norm(bottom_front_3d - height_3d * n + radius_3d * backwards_vector_3d)
                    height_3d *= height_depth / old_height_depth
                    logger.debug(f"> Improved height: {height_3d:.5f}")
                    radius_3d *= radius_depth / old_radius_depth
                    logger.debug(f"> Improved radius: {radius_3d:.5f}=={radius_3d*2:.5f}d")
                    old_height_depth = height_depth
                    old_radius_depth = radius_depth

                # improve bottom_front_3d estimation
                def get_bottom_front_error_side(p):
                    bottom_front_3d_new = bottom_front_3d + p * side_vector_3d
                    glass_top_left_point = bottom_front_3d_new + radius_3d * backwards_vector_3d - height_3d * n - radius_3d * side_vector_3d
                    x, y, z = self.camera_intrinsics @ glass_top_left_point
                    x_tl, y_tl = x / z, y / z
                    glass_top_right_point = bottom_front_3d_new + radius_3d * backwards_vector_3d - height_3d * n + radius_3d * side_vector_3d
                    x, y, z = self.camera_intrinsics @ glass_top_right_point
                    x_tr, y_tr = x / z, y / z
                    glass_top_front_point = bottom_front_3d_new - height_3d * n
                    x, y, z = self.camera_intrinsics @ glass_top_front_point
                    x_tf, y_tf = x / z, y / z
                    return np.linalg.norm(np.array([x_tl, y_tl]) - top_left_2d) + \
                        np.linalg.norm(np.array([x_tr, y_tr]) - top_right_2d) + \
                        np.linalg.norm(np.array([x_tf, y_tf]) - top_front_2d)

                p = fsolve(get_bottom_front_error_side, 0.0)  # initial guess is zero offset
                logger.debug(f"-> Bottom front offset: {p}")
                bottom_front_3d = bottom_front_3d + p * side_vector_3d

                def get_radius_error(r):
                    glass_top_left_point = bottom_front_3d + r * backwards_vector_3d - height_3d * n - r * side_vector_3d
                    x, y, z = self.camera_intrinsics @ glass_top_left_point
                    x_tl, y_tl = x / z, y / z
                    glass_top_right_point = bottom_front_3d + r * backwards_vector_3d - height_3d * n + r * side_vector_3d
                    x, y, z = self.camera_intrinsics @ glass_top_right_point
                    x_tr, y_tr = x / z, y / z
                    glass_top_middle_point = bottom_front_3d + r * backwards_vector_3d - height_3d * n
                    x, y, z = self.camera_intrinsics @ glass_top_middle_point
                    x_tm, y_tm = x / z, y / z
                    glass_top_front_point = bottom_front_3d - height_3d * n
                    x, y, z = self.camera_intrinsics @ glass_top_front_point
                    x_tf, y_tf = x / z, y / z
                    top_middle_2d = (top_left_2d + top_right_2d) / 2
                    return np.linalg.norm(np.array([x_tl, y_tl]) - np.array([x_tr, y_tr])) - width_2d + \
                        np.linalg.norm(np.array([x_tm, y_tm]) - np.array([x_tf, y_tf])) - np.linalg.norm(
                            top_middle_2d - top_front_2d)

                new_radius_3d = fsolve(get_radius_error, radius_3d)
                logger.debug(f"-> new radius: {new_radius_3d[0]:.5f}=={new_radius_3d[0]*2:.5f}d")
                # sanity check if r_sol is correct
                if 0.005 < new_radius_3d[0] < 0.5:  # glasses are usually not wider than 1m
                    radius_3d = new_radius_3d[0]
                else:
                    logger.debug("Radius estimation improvement failed!")

                def opt_shape(p):
                    glass_angle = p[0]
                    height_3d = p[1]
                    x, y, z = self.camera_intrinsics @ bottom_front_3d
                    x_bf, y_bf = x / z, y / z
                    glass_top_front_point = bottom_front_3d - height_3d * n - np.tan(glass_angle) * height_3d * backwards_vector_3d
                    x, y, z = self.camera_intrinsics @ glass_top_front_point
                    x_tf, y_tf = x / z, y / z
                    glass_top_middle_point = glass_top_front_point + radius_3d * backwards_vector_3d
                    x, y, z = self.camera_intrinsics @ glass_top_middle_point
                    x_tm, y_tm = x / z, y / z
                    glass_top_left_point = glass_top_middle_point - radius_3d * side_vector_3d
                    x, y, z = self.camera_intrinsics @ glass_top_left_point
                    x_tl, y_tl = x / z, y / z
                    glass_top_right_point = glass_top_middle_point + radius_3d * side_vector_3d
                    x, y, z = self.camera_intrinsics @ glass_top_right_point
                    x_tr, y_tr = x / z, y / z
                    return (np.linalg.norm(np.array([x_tl, y_tl]) - top_left_2d) + \
                        np.linalg.norm(np.array([x_tr, y_tr]) - top_right_2d) + \
                        np.linalg.norm(np.array([x_tm, y_tm]) - top_middle_2d) + \
                        np.linalg.norm(np.array([x_tf, y_tf]) - top_front_2d),

                            (np.deg2rad(3) - p[0]) ** 2 / 2) # avoid too much tilt

                p = fsolve(opt_shape, np.array([np.deg2rad(3), height_3d]))  # most glasses have a slight angle
                logger.debug(f"-> Glass angle: {np.rad2deg(p[0]): .5f}, height: {p[1]: .5f} ({p[1] - height_3d})")
                if 0.0 < p[0] < np.deg2rad(10) and 0.0 < height_3d < 0.3:  # glasses are usually not tilted more than 10 degrees + height is usually not more than 30 cm
                    glass_angle = p[0]
                    height_3d = p[1]
                else:
                    logger.debug("Shape optimization failed!")
                    glass_angle = 0.0

                # sanity check radius and height
                if not (0.005 < radius_3d < 0.5 and 0.005 < height_3d < 0.5):
                    logger.debug("Radius or height estimation failed!")
                    continue

                top_middle_3d = bottom_front_3d + radius_3d * backwards_vector_3d - height_3d * n - np.tan(glass_angle) * height_3d * backwards_vector_3d
                top_left_3d = bottom_front_3d + radius_3d * backwards_vector_3d - height_3d * n - np.tan(glass_angle) * height_3d * backwards_vector_3d - radius_3d * side_vector_3d
                top_right_3d = bottom_front_3d + radius_3d * backwards_vector_3d - height_3d * n - np.tan(glass_angle) * height_3d * backwards_vector_3d + radius_3d * side_vector_3d
                top_front_3d = bottom_front_3d - height_3d * n - np.tan(glass_angle) * height_3d * backwards_vector_3d

                glasses.append([top_middle_3d, radius_3d, height_3d, glass_angle, bb[0], keypoints_o])

                def get_final_error():
                    glass_top_left_point = bottom_front_3d + radius_3d * backwards_vector_3d - height_3d * n - np.tan(glass_angle) * height_3d * backwards_vector_3d - radius_3d * side_vector_3d
                    x, y, z = self.camera_intrinsics @ glass_top_left_point
                    x_tl, y_tl = x / z, y / z
                    glass_top_right_point = bottom_front_3d + radius_3d * backwards_vector_3d - height_3d * n - np.tan(glass_angle) * height_3d * backwards_vector_3d + radius_3d * side_vector_3d
                    x, y, z = self.camera_intrinsics @ glass_top_right_point
                    x_tr, y_tr = x / z, y / z
                    glass_top_front_point = bottom_front_3d - height_3d * n - np.tan(glass_angle) * height_3d * backwards_vector_3d
                    x, y, z = self.camera_intrinsics @ glass_top_front_point
                    x_tf, y_tf = x / z, y / z
                    x, y, z = self.camera_intrinsics @ bottom_front_3d
                    x_bf, y_bf = x / z, y / z
                    return np.linalg.norm(np.array([x_tl, y_tl]) - top_left_2d) + \
                        np.linalg.norm(np.array([x_tr, y_tr]) - top_right_2d) + \
                        np.linalg.norm(np.array([x_tf, y_tf]) - top_front_2d) + \
                        np.linalg.norm(np.array([x_bf, y_bf]) - bottom_front_2d)
                if X_Platform_Camera is not None:
                    final_error = get_final_error()
                    logger.debug(f"Localized glass: {top_middle_3d}, {radius_3d}=={radius_3d*2}d, {height_3d} -> {X_Platform_Camera @ np.append(top_middle_3d, 1)} (error: {final_error})")

        # do a check if we detected double glasses
        glasses_ret = []
        meshes_ret = []
        for i, glass in enumerate(glasses):
            glass_r = glass[1]
            glass_single = True
            for j, other_glass in enumerate(glasses):
                other_glass_r = other_glass[1]
                if i != j and i < j:
                    if np.linalg.norm(glass[0] - other_glass[0]) < (glass_r + other_glass_r)/2.0:
                        logger.debug(f"Detected double glass at {glass[0]}")
                        glass_single = False
                        break
            if glass_single:
                glasses_ret.append(glass)
        # finally sort the glasses from distance to np.array([-1.0, 0.0, table_height-platform_height])
        glasses_ret = sorted(glasses_ret, key=lambda glass: np.linalg.norm((X_Platform_Camera @ np.append(glass[0], 1))[:3] - np.array([0.0, 1.0, table_height-platform_height])))

        return glasses_ret