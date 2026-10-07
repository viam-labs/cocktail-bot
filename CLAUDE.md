# CLAUDE.md

## Working in this repo

- Default to no code comments; add terse one-liners only when the WHY (not the WHAT) is non-obvious from the code.
- Do NOT bandaid over upstream bugs or platform gaps in any dependency — raise them immediately with a severity (critical / high / medium / low).

## Platform shortcomings

Known gaps this module cannot fix on its own. Do not paper over them — reference this section when you hit them and, if new ones surface, add them here.

- **high**: SPILL's four keypoints can't determine a glass's wall angle (a one-parameter family of glasses reprojects exactly). `spill-glass-finder` takes it as the `tilt_deg` attribute (default 3°, as SPILL effectively does). A wrong angle Δθ moves the rim center by about h·tan(Δθ) horizontally and shifts the height.
- **medium**: Python SDK resources get no handle to the frame system (`Module.parent` is internal). `spill-glass-finder` therefore fits the table plane in the camera frame and returns camera-frame results for callers to transform.
- **low**: `tlpss/keypoint-detection` is not on PyPI and pins `pytorch-lightning<=1.9.4`, `wandb` and `fiftyone`, so `spill-glass-finder` vendors its model and peak extraction. Current timm no longer persists MaxViT's `relative_position_index` buffers, which `wild_glasses.ckpt` contains; the loader checks they equal the recomputed buffers before dropping them.
