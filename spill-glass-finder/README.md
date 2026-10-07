# spill-glass-finder

`viam:cocktail-bot:spill-glass-finder` (`rdk:service:vision`, module `viam:spill-glass-finder`) finds transparent
glasses and fits each one's rim center, radius and height in the camera frame. It ports the glass localization
from [SPILL](https://github.com/Louadria/SPILL) (Adriaens et al., "SPILL: Size, Pose, and Internal Liquid Level
Estimation of Transparent Glassware for Robotic Bartending", MIT license).

Per call it takes one color image, one point cloud and the intrinsics from `camera_name`, then:

1. gets 2D boxes from `detector_name` (e.g. the robot's `yolov8` service with COCO weights) and keeps `labels` at
   or above `min_confidence`, highest score first;
2. pads each box by `crop_padding` on every side, crops, resizes to 256x256 (aspect not preserved) and runs SPILL's
   MaxViT-UNet keypoint model (`wild_glasses.ckpt`) on CPU, one batch per frame; the best heatmap peak of channels
   0-3 gives bottom_front, top_front, top_left, top_right (the fluid-level channel is ignored); a glass missing any
   of them is skipped;
3. fits the table plane to the point cloud (RANSAC + SVD, camera frame);
4. intersects the bottom_front ray with the table and solves rim radius, height and the base's sideways offset from
   the four keypoints. Depth is never read on the glass itself; it is transparent;
5. drops a glass whose rim center is within the mean radius of a higher-scoring glass's rim center.

Distances are in mm in the camera frame. Depth must be registered to the color image with the same intrinsics.

## Config

```json
{
  "name": "spill-glass-finder",
  "api": "rdk:service:vision",
  "model": "viam:cocktail-bot:spill-glass-finder",
  "attributes": {
    "camera_name": "cam",
    "detector_name": "yolov8",
    "checkpoint_path": "/opt/spill/wild_glasses.ckpt"
  }
}
```

| attribute | default | notes |
|---|---|---|
| `camera_name` | required | RGB-D camera. Depth must be pixel-aligned with color |
| `detector_name` | required | 2D detector vision service |
| `checkpoint_path` | required | `wild_glasses.ckpt` on the machine (`scripts/fetch_weights.sh` downloads it and checks its sha256) |
| `labels` | `["wine glass", "cup", "vase"]` | SPILL's COCO classes |
| `min_confidence` | `0.5` | |
| `crop_padding` | `0.25` | SPILL's 64/256 |
| `table_max_depth_mm` | `2000` | cloud points with a larger z are ignored when fitting the table |
| `table_ransac_threshold_mm` | `5` | |
| `table_offset_mm` | `0` | moves the plane along its up normal; SPILL used -5 |
| `surface_points` | `2000` | points sampled on each fitted glass |
| `channel_order` | `"rgb"` | channel order fed to the keypoint model (see below) |
| `tilt_deg` | `3` | assumed wall angle; it can't be measured from the keypoints (see below) |

## Outputs

- `GetDetections[FromCamera]`: per glass, its detector box labelled `"{class} r={r}mm h={h}mm"`, then four ±4 px
  boxes at the keypoints labelled `kp-bottom_front`, `kp-top_front`, `kp-top_left` and `kp-top_right` so they show on the
  camera view. `GetDetections(image)` uses the given image with the camera's current point cloud.
- `GetObjectPointClouds`: per glass, points sampled on the fitted surface (side wall from base to rim, plus the rim
  circle), camera frame. Its geometry is a 5 mm sphere at the rim center with the label above, and its reference frame
  is `camera_name`. With `extra={"debug": true}` it also returns, per glass, the camera's raw points inside the box
  (label suffix `-raw`).
- `CaptureAllFromCamera`: image, detections and objects from one capture. Classifications and 3D detections are
  not supported.
- `DoCommand({"find_glasses": {}})`, the contract the bartender and `cocktail-cli hover-glass --glass-finder` use:

```json
{
  "glasses": [
    {
      "label": "wine glass", "score": 0.91, "bbox": [x0, y0, x1, y1],
      "keypoints_px": {"bottom_front": [u, v], "top_front": [u, v], "top_left": [u, v], "top_right": [u, v]},
      "rim_center_mm": [x, y, z], "base_front_mm": [x, y, z],
      "radius_mm": 38.2, "height_mm": 181.0, "tilt_deg": 3.0, "reprojection_rmse_px": 0.7,
      "frame": "cam"
    }
  ],
  "table": {"normal": [nx, ny, nz], "d_mm": 612.0, "inlier_ratio": 0.55}
}
```

Glasses are sorted by score, highest first. The table is `normal·p + d_mm = 0`. `normal` is a unit vector that points
up, toward the camera. Point clouds on the wire are PCD in meters, as with every Viam camera; RDK converts to mm.

## Differences from SPILL

**Table plane in the camera frame.** SPILL takes the mode of a histogram of point heights in the platform frame,
which needs the camera's pose. The Python SDK gives a resource no handle to the frame system: the module's parent
`RobotClient`, which has `transform_pose`, is internal to `Module`. So this module fits the dominant plane within
`table_max_depth_mm` directly in the camera frame. It returns camera-frame results, and the caller transforms them
(as `cocktail-cli hover-glass --glass-finder` does). It orients the normal toward the camera and rejects planes
with fewer than 5% inliers, or planes within 5° of the optical axis. It assumes the table is the largest plane in
range. A wall or floor that dominates the view will be picked instead.

**One least-squares problem instead of chained `fsolve`.** SPILL runs a fixed-point size guess and then three
`fsolve` stages: base side offset, radius, then (tilt, height). Each stage freezes the others, and `fsolve` is a
root-finder applied to sums of norms. This module keeps SPILL's initial guess. It then solves the side offset,
radius and height in one bounded `scipy.optimize.least_squares`. The residuals are the x/y reprojection errors of
all four keypoints. The bounds are radius 5-500 mm and height 5-300 mm, and a glass whose solution lands on a bound
is dropped. SPILL's top_middle-vs-midpoint residual is left out. Under perspective the projected rim center is not
the midpoint of the projected rim extremes, so that residual biases the fit. The midpoint of the projections adds
nothing beyond the top_left and top_right residuals.

**Tilt is an input, not an estimate.** In SPILL's model the wall angle θ sets the base radius to `r - h·tanθ`.
top_front, top_left and top_right all lie on the horizontal rim circle. Scaling that circle about the camera center
keeps their projections, and for every scale some (h, θ) still puts the base front on its ray. So the four keypoints
fit a one-parameter family of glasses exactly. SPILL's `fsolve` pins θ at 3°, since its second equation is
`θ - 3° = 0`. Here θ is the `tilt_deg` attribute. If the real wall angle differs by Δθ, the rim center moves about
`h·tan(Δθ)` horizontally, which is 8 mm for a 150 mm glass at Δθ = 3°. Height also moves, by about 2-28 mm in the
synthetic sweep (more for tall glasses and steep views).

**Dedup keeps the higher-scoring glass.** SPILL keeps the later detection, which is the lower-scoring one.

## Channel order evidence

SPILL feeds the keypoint model BGR at inference: OpenCV images in `glassloc.py`, and Gradio RGB converted to BGR in
the HF Space's `app.py`. The training loader in `tlpss/keypoint-detection` (`skimage.io.imread` + `ToTensor`) feeds
it RGB. On the 310 Glasses-in-the-Wild validation crops, with GT keypoints and min peak distance 5, both orders give
essentially the same result:

| order | all 4 found | mean error bf / tf / tl / tr (px) | median error (px) |
|---|---|---|---|
| rgb | 308/310 | 4.03 / 2.58 / 1.92 / 2.26 | 2.58 / 1.79 / 1.45 / 1.37 |
| bgr | 308/310 | 4.20 / 2.65 / 1.88 / 2.25 | 2.88 / 1.72 / 1.41 / 1.40 |

The paired difference (rgb - bgr) is -0.04 ± 0.06 px (SEM, n = 1238). The default is `rgb`, which matches training
and needs no conversion from the camera's RGB.

## CPU latency

Measured on an Apple M2 Pro with torch 2.14 CPU and 6 threads, timing the keypoint model only:

| batch | ms / batch | ms / crop |
|---|---|---|
| 1 | 128 | 128 |
| 2 | 278 | 139 |
| 4 | 356 | 89 |
| 8 | 527 | 66 |

Loading the checkpoint takes about 1 s. The table fit takes about 110 ms on 270k points, and each glass solve about
1.5 ms. Expect a robot's ARM CPU to be several times slower.

## Synthetic accuracy (tests/test_geometry.py)

The sweep covers glasses with radius 25-45 mm, height 60-200 mm and tilt 0-8° (tilt known). The base front is
300-1000 mm from the camera, and the camera looks down 30-70° with 5° roll. With noise-free keypoints the solver
recovers the rim center, radius and height to well under 1 mm. With σ = 1 px noise on every keypoint coordinate, the
95th-percentile errors are:

| range | rim center | radius | height |
|---|---|---|---|
| 300 mm | 1.6 mm | 0.4 mm | 1.0 mm |
| 600 mm | 3.4 mm | 0.9 mm | 2.9 mm |
| 1000 mm | 7.3 mm | 1.2 mm | 5.7 mm |

The test asserts p95 below 1% of range for rim center and height, and below 2 mm for radius.

## Development

```bash
./setup.sh                     # .venv with CPU torch
scripts/fetch_weights.sh       # weights/wild_glasses.ckpt (gitignored)
.venv/bin/python -m pip install pytest
.venv/bin/python -m pytest     # the real-model test is skipped without the weights
./build.sh                     # module.tar.gz
```

The real-model test writes an annotated image to `tests/output/real_model_keypoints.png`. Its fixtures are three
Glasses-in-the-Wild crops (CC BY 4.0, see `tests/fixtures/glasses_in_the_wild/README.md`).

`src/spill_glass_finder/model.py` vendors the MaxViT-UNet detector and the heatmap peak extraction from
[tlpss/keypoint-detection](https://github.com/tlpss/keypoint-detection) (MIT). That package isn't on PyPI and pins
`pytorch-lightning<=1.9.4`, `wandb` and `fiftyone`. The checkpoint loads with `torch.load(weights_only=True)`.
Current timm no longer stores MaxViT's `relative_position_index` buffers, so the loader checks that the
checkpoint's copies equal the recomputed ones and then drops them.

## Known limitations

- The wall angle is assumed, not measured (see above). This is the dominant error for tall or flared glasses.
- SPILL's model puts top_left/top_right at `rim_center ± r·side`. The true silhouette tangents of a circle under
  perspective differ by about `(r/D)²/2` relative, which is below 0.5% at r/D < 0.1.
- The keypoint model can mistake a water line or the far rim for the near rim on side-on views. One of the three
  fixtures shows this: top_front and top_right land about 26 px low.
- Heatmap peaks are integer pixels in the 256x256 crop. That limits keypoint resolution to about one crop pixel
  (box size / 256 in image pixels).
- Table fitting assumes the table is the dominant plane within range.
- The glass must stand on the fitted table plane. Stacked or held glasses come out wrong.
- PCD `binary_compressed` clouds are rejected.
- The image and point cloud come from separate `GetImages` and `GetPointCloud` calls. They can be from different
  frames if the scene or camera moves between the two calls.
