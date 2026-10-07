# CLAUDE.md

## Working in this repo

- Default to no code comments; add terse one-liners only when the WHY (not the WHAT) is non-obvious from the code.
- Do NOT bandaid over upstream bugs or platform gaps in any dependency — raise them immediately with a severity (critical / high / medium / low).

## Platform shortcomings

Known gaps this module cannot fix on its own. Do not paper over them — reference this section when you hit them and, if new ones surface, add them here.

- **high**: SPILL pins the glass wall tilt θ ≈ 3° through the prior in `opt_shape` (its second equation is `(3° − θ)²/2`). The four keypoints can't observe θ: a one-parameter family of glasses reprojects exactly. If a real wall angle is off by Δθ, the rim center moves about h·tan(Δθ) horizontally. `spill-glass-finder` keeps this verbatim, with no workaround.
- **medium**: Python SDK resources have no frame-system access. There is no `$framesystem` client, and `Module.parent` is private. `spill-glass-finder` therefore reads a static `camera_pose_in_world` from config to build SPILL's `X_Platform_Camera`. It is wrong whenever the camera moves (e.g. wrist-mounted) or the frame config changes without updating it.
- **low**: `tlpss/keypoint-detection` is not on PyPI, pins `pytorch-lightning<=1.9.4`, `wandb` and `fiftyone`, and only loads `wild_glasses.ckpt` strictly on torch 2.2. On current torch, MaxViT's `relative_position_index` buffers are non-persistent. `spill-glass-finder` therefore vendors its model, loader and peak extraction. The loader uses `weights_only=True` and `pretrained=False`, and drops those buffers only after checking they equal the recomputed ones.
- **low**: upstream SPILL quirks kept verbatim in `spill-glass-finder`:
  - `localize_glass` raises IndexError when a crop has no peak in channels 0–3.
  - Image keypoints are truncated to whole pixels (int array).
  - The post-`fsolve` sanity check tests the old height.
  - Glasses are sorted by distance to SPILL's platform point (0, 1 m, table).
