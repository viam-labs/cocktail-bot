# CLAUDE.md

## Working in this repo

- Default to no code comments; add terse one-liners only when the WHY (not the WHAT) is non-obvious from the code.
- Do NOT bandaid over upstream bugs or platform gaps in any dependency — raise them immediately with a severity (critical / high / medium / low).

## Platform shortcomings

Known gaps this module cannot fix on its own. Do not paper over them — reference this section when you hit them and, if new ones surface, add them here.

- **high**: SPILL's four keypoints can't determine a glass's wall tilt: a one-parameter family of glasses reprojects exactly. SPILL's last `fsolve` effectively pins tilt at 3°, and `spill-glass-finder` keeps that verbatim. A real wall angle off by Δθ moves the rim center about h·tan(Δθ) horizontally.
- **medium**: the Python SDK gives resources no way to read the frame system. There is no `$framesystem` client, and `Module.parent` is private. `spill-glass-finder` therefore takes a static `camera_pose_in_world`, which must be kept in sync with the machine's frame config by hand.
- **medium**: `tlpss/keypoint-detection` (used verbatim by SPILL) is not on PyPI. Its `load_from_checkpoint` only loads `wild_glasses.ckpt` strictly on an old stack: torch 2.2.2, torchvision 0.17.2, timm 0.9.16, pytorch-lightning 1.9.4, numpy<2, setuptools<81 and huggingface_hub<0.30. With torch 2.14 the `relative_position_index` buffers are non-persistent and the load fails. It also downloads ImageNet MaxViT weights from the Hugging Face Hub on first start (`pretrained=True`), so the machine needs network or a pre-filled cache.
- **low**: upstream SPILL quirks kept verbatim in `spill-glass-finder`:
  - `localize_glass` raises IndexError when a crop has no peak in channels 0–3.
  - Image keypoints are truncated to whole pixels (int array).
  - The post-`fsolve` sanity check tests the old height.
  - Glasses are sorted by distance to SPILL's platform point (0, 1 m, table).
