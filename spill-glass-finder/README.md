# spill-glass-finder

`viam:cocktail-bot:spill-glass-finder` (`rdk:service:vision`, module `viam:spill-glass-finder`) finds transparent
glasses and returns each one's rim center, radius, height and tilt in the world frame. The algorithm is
[SPILL](https://github.com/Louadria/SPILL)'s glass localization (Adriaens et al., "SPILL: Size, Pose, and Internal
Liquid Level Estimation of Transparent Glassware for Robotic Bartending", MIT license). It is vendored verbatim from
`glassloc/glassloc.py` and `glassloc/GlassDetector.py` at commit `7fe7282d29730c6026d3dd682c6d926ee3f52735`, in
`src/spill_glass_finder/spill/`.

Per call the module takes one color image, one point cloud and the intrinsics from `camera_name`, then:

1. SPILL's `localize_table` finds the table height. It transforms the cloud to the world frame, keeps heights in
   `[table_crop_height_min_m, table_crop_height_max_m]`, takes the fullest of 500 histogram bins and subtracts 5 mm.
2. SPILL's `localize_glass` does the rest:
   - It gets 2D boxes from `detector_name` (the robot's `yolov8` service, standing in for SPILL's in-process YOLO).
   - It pads each box by 64/256, crops, resizes to 256x256 and runs SPILL's keypoint model (`wild_glasses.ckpt`,
     vendored MaxViT-UNet, CPU) on the BGR crop.
   - From the four structural keypoints it solves radius, height and tilt with SPILL's fixed-point loop and three
     `fsolve` stages, against the table plane.
   - It drops duplicate glasses and sorts the rest SPILL's way.

Fluid level is not used.

Viam point clouds and outputs are in mm, while SPILL works in meters; the conversion happens only at the module
boundary. SPILL's platform frame is the Viam world frame with `platform_height = 0`, so world z must point up.

## Config

```json
{
  "name": "spill-glass-finder",
  "api": "rdk:service:vision",
  "model": "viam:cocktail-bot:spill-glass-finder",
  "attributes": {
    "camera_name": "cam",
    "detector_name": "yolov8",
    "checkpoint_path": "/opt/spill/wild_glasses.ckpt",
    "camera_pose_in_world": {
      "translation": {"x": 0, "y": 0, "z": 1250},
      "orientation": {"type": "ov_degrees", "value": {"x": 0, "y": 1, "z": -1, "th": 0}}
    }
  }
}
```

| attribute | default | notes |
|---|---|---|
| `camera_name` | required | RGB-D camera. Depth must be pixel-aligned with color |
| `detector_name` | required | 2D detector vision service |
| `checkpoint_path` | required | `wild_glasses.ckpt` on the machine (`scripts/fetch_weights.sh` downloads it and checks its sha256) |
| `camera_pose_in_world` | required | The camera frame's pose in world, in the Viam frame-config shape (translation in mm, `ov_degrees` orientation). It must be the pose relative to **world**: if the camera's frame parent is not world, compose the chain yourself |
| `labels` | `["cup", "vase", "wine glass"]` | SPILL's classes |
| `min_confidence` | `0.5` | detector boxes below this are ignored |
| `surface_points` | `2000` | points sampled on each glass's cylinder |
| `table_crop_height_min_m` | `0.3` | SPILL's band of world heights searched for the table |
| `table_crop_height_max_m` | `1.3` | |

**The table must lie inside the crop band.** SPILL's 0.3–1.3 m band assumes the platform origin is at floor level.
If our world origin sits at the arm base, possibly on the table, the table is near z = 0 and falls outside the band.
`localize_table` then picks whatever else is in the band, or raises if nothing is. In that case set the band
around the table height in your world frame, e.g. `-0.2` / `0.8`.

### Why a static camera pose

The Python SDK offers no supported way for a module's resource to read the frame system. Go modules get it as the
`$framesystem` dependency, but the Python SDK has no client for that service. The module's parent `RobotClient`,
which has `transform_pose`, is private to `viam.module.module.Module` and is never handed to resources. So the
camera's world pose comes from config and must be kept in sync with the machine's frame system by hand. See
"Platform shortcomings" in the repo's CLAUDE.md.

## Outputs

All 3D outputs are in the **world** frame, in mm.

- `DoCommand({"find_glasses": {}})`, the contract the bartender and `cocktail-cli hover-glass --glass-finder` use:

```json
{
  "glasses": [
    {
      "label": "wine glass", "score": 0.91, "bbox": [x0, y0, x1, y1],
      "keypoints_px": {"bottom_front": [u, v], "top_front": [u, v], "top_left": [u, v], "top_right": [u, v]},
      "rim_center_mm": [x, y, z], "radius_mm": 38.2, "height_mm": 181.0, "tilt_deg": 3.0,
      "frame": "world"
    }
  ],
  "table": {"height_mm": 745.0, "frame": "world"}
}
```

  Glasses come in SPILL's order: duplicates removed, then sorted by distance from the world point (0, 1 m, table
  height). The keypoints are SPILL's image-space keypoints, which SPILL truncates to whole pixels.
- `GetDetections[FromCamera]`: per glass, its detector box labelled `"{class} r={r}mm h={h}mm"`, then four ±4 px
  boxes at the keypoints labelled `kp-bottom_front`, `kp-top_front`, `kp-top_left` and `kp-top_right`.
- `GetObjectPointClouds`: per glass, points on an upright cylinder of the fitted radius hanging `height` below the
  rim center. This is the cylinder SPILL's visualizer draws: 80% side wall, 20% rim circle. Reference frame is
  `world`, and the geometry is a 5 mm sphere at the rim center. With `extra={"debug": true}` it also returns, per
  glass, the camera's raw points inside the box, transformed to world (label suffix `-raw`).
- `CaptureAllFromCamera`: image, detections and objects from one capture.

## Vendored code and its differences from upstream

`tests/test_spill_equivalence.py` loads the original files from a SPILL checkout (`SPILL_REPO`, default
`/Users/robin/bov/SPILL`) with their airo, open3d and visualizer imports stubbed. It asserts that the vendored
`localize_glass`, `localize_table`, `get_glass_bounding_boxes` and `keypoint_detector_local_inference` return
identical numbers on the same inputs, with exact float equality. The test skips when no checkout is present.

Line-level changes, `glassloc.py`:
- The MIT header is added.
- The `open3d`, `matplotlib`, `airo_*` and `CameraVisualizer` imports are removed. `GlassDetector` is imported
  relatively, and a module logger is added.
- `__init__(camera_intrinsics, visualize=False)` becomes `__init__(camera_intrinsics, checkpoint_name, classes,
  detector)`. The hard-coded `checkpoint_name` and `classes` become these parameters, the visualizer setup is
  removed, and `save_image` is removed.
- `localize_table` takes `crop_height_min=0.3, crop_height_max=1.3` as parameters instead of local constants.
  `point_cloud_to_open3d(point_cloud).transform(X).translate(...)` / `.to_legacy().points` becomes the same affine
  transform in numpy, on an (N, 3) array in meters.
- `OpenCVIntImageType`, `CameraIntrinsicsMatrixType` and `PointCloud` hints become `np.ndarray`.
- Every `if self._camera_visualizer is not None:` block is removed, as is the open3d mesh building inside them.
- `print(...)` becomes `logger.debug(...)`. Commented-out prints are left as they are.
- Fluid level is removed. That covers the channel-4 branch, `fluid_level_2d` and the fluid percentage. The
  completeness check `len(keypoints_o) - int(fluid_level_detected) - int(fluid_2nd_level_detected) < 4` becomes
  `len(keypoints_o) < 4`, which is equivalent without fluid points.
- `glasses.append([top_middle_3d, radius_3d, height_3d])` becomes
  `glasses.append([top_middle_3d, radius_3d, height_3d, glass_angle, bb[0], keypoints_o])`. Dedup and sort only
  read `glass[0]` and `glass[1]`, so they are unaffected. The extra fields carry tilt, box and keypoints to the
  outputs.

`GlassDetector.py`:
- The MIT header is added, and the `airo_*` and `ultralytics` imports are removed.
- `__init__` takes a `detector` argument in place of `YOLO("checkpoints/yolov8m.pt").to("cuda")`. It is
  `detections.FrameDetections`, which presents the vision service's boxes in the ultralytics result shape that
  `get_glass_bounding_boxes` reads: float32 `xyxyn`, `cls`, `names`, highest score first.
- `.cuda()` becomes `.cpu()`, and the `device` default changes from `"cuda"` to `"cpu"`.
- The `OpenCVIntImageType` hint becomes `np.ndarray`.

- `from keypoint_detection.utils.heatmap import get_keypoints_from_heatmap_batch_maxpool` and
  `from keypoint_detection.utils.load_checkpoints import get_model_from_wandb_checkpoint, load_from_checkpoint`
  become imports of the vendored copies in `spill_glass_finder/model.py` (`get_model_from_wandb_checkpoint` is
  unused upstream and dropped).

## Keypoint model (vendored, not the package)

`src/spill_glass_finder/model.py` vendors the MaxViT-UNet detector, `load_from_checkpoint` and
`get_keypoints_from_heatmap_batch_maxpool` from [tlpss/keypoint-detection](https://github.com/tlpss/keypoint-detection)
(MIT). Peak extraction keeps the package defaults (max 20 peaks, min distance 1 px, threshold 0.01). The package is
not used directly, for three reasons:
- It isn't on PyPI and pins `pytorch-lightning<=1.9.4`, `wandb` and `fiftyone`.
- Its strict `load_state_dict` of `wild_glasses.ckpt` only works on the old torch 2.2.2 / torchvision 0.17.2 stack.
  Current torch no longer persists MaxViT's `relative_position_index` buffers inside the feature extractor.
- It builds the backbone with `pretrained=True`, which downloads ImageNet weights from the Hugging Face Hub on
  every fresh machine.

The vendored loader works around these as follows:
- It reads the checkpoint with `torch.load(weights_only=True)`, so no pickled code runs.
- It builds the backbone with `pretrained=False`.
- It drops the checkpoint's `relative_position_index` buffers, but only after checking that they equal the ones the
  current model recomputes.

I checked the vendored loader against the real package (torch 2.2.2 stack) on the three real-image fixtures. The
keypoints and errors vs labels were identical.

## Channel order

SPILL's `localize_glass` takes an `OpenCVIntImageType` (BGR) and feeds it to `to_tensor` unchanged, so the module
converts the decoded RGB image to BGR right after decoding. The model was trained on RGB (the package's dataset
loader uses `skimage.io.imread`), so I checked whether the order matters. On the 310 Glasses-in-the-Wild validation
crops with their labelled keypoints:

| order | all 4 found | mean error bf / tf / tl / tr (px) | median error (px) |
|---|---|---|---|
| rgb | 308/310 | 4.03 / 2.58 / 1.92 / 2.26 | 2.58 / 1.79 / 1.45 / 1.37 |
| bgr | 308/310 | 4.20 / 2.65 / 1.88 / 2.25 | 2.88 / 1.72 / 1.41 / 1.40 |

The paired difference (rgb - bgr) is -0.04 ± 0.06 px (SEM, n = 1238). There is no significant difference, so
following SPILL's BGR costs nothing. That comparison used min peak distance 5; the module uses SPILL's default of 1,
which only changes which border pixels are excluded.

## Tilt is not observable

SPILL pins θ ≈ 3° through the prior in `opt_shape`: its second equation is `(3° - θ)²/2 = 0`. The four keypoints
can't observe θ. top_front, top_left and top_right all lie on the horizontal rim circle, and scaling that circle
about the camera center keeps their projections. For every scale, some (height, θ) still puts the base front on
its ray, so a one-parameter family of glasses reprojects exactly.

If a real glass's wall angle differs from 3° by Δθ, the rim center moves about h·tan(Δθ) horizontally. That is
about 8 mm for a 150 mm glass at Δθ = 3°. Height moves too, by about 2-28 mm in a synthetic sweep, more for tall
glasses and steep views. No code works around this.

## CPU latency

Measured on an Apple M2 Pro with torch 2.14 CPU and 6 threads:

| stage | time |
|---|---|
| checkpoint load | ~0.9 s, once |
| keypoint model per glass (SPILL runs one crop at a time) | ~140 ms |
| `localize_table`, 290k points | ~9 ms |
| `localize_glass` geometry per glass | ~2 ms |

Expect a robot's ARM CPU to be several times slower.

## Development

```bash
./setup.sh                     # .venv with CPU torch
scripts/fetch_weights.sh       # weights/wild_glasses.ckpt (gitignored)
.venv/bin/python -m pip install pytest
.venv/bin/python -m pytest     # the real-model test skips without weights; the equivalence tests without a SPILL checkout
./build.sh                     # module.tar.gz
```

`tests/test_real_model.py` runs the real checkpoint on three Glasses-in-the-Wild crops (CC BY 4.0, see
`tests/fixtures/glasses_in_the_wild/README.md`). It checks that the vendored and original
`keypoint_detector_local_inference` agree, and writes `tests/output/real_model_keypoints.png`.

## Known limitations (upstream behavior, kept as is)

- Tilt is assumed (≈3°), not measured; see above.
- `keypoints_o` is built from integer heatmap peaks, so mapping it back to the image truncates to whole pixels.
- If a crop has no peak at all in channels 0–3, `keypoints_o[:, 0]` raises IndexError and the whole call fails. The
  0.01 peak threshold makes this rare.
- After the last `fsolve`, the sanity check tests the *previous* `height_3d`, not the new `p[1]`.
- The final sort is by distance to the platform point (0, 1 m, table height), a point from SPILL's own setup.
- The keypoint model can mistake a water line for the rim on side-on views. One fixture shows top_front and
  top_right about 26 px low.
- The image and the point cloud come from separate camera calls.
- PCD `binary_compressed` clouds are rejected.
