# cocktail-bot

## Models

- `viam:cocktail-bot:bartender` (`rdk:service:generic`): order queue and drink orchestration.
- `viam:cocktail-bot:order-sensor` (`rdk:component:sensor`): one reading per order for Data Management.
- `viam:cocktail-bot:glass-finder` (`rdk:service:vision`): locates glasses in 3D. Runs a 2D detector on the camera image, keeps the points of the camera point cloud that project inside each glass box, and returns them in the world frame.

```json
{
  "name": "glass-finder",
  "api": "rdk:service:vision",
  "model": "viam:cocktail-bot:glass-finder",
  "attributes": {
    "camera_name": "cam",
    "detector_name": "yolov8",
    "labels": ["wine glass", "cup"],
    "min_confidence": 0.5
  }
}
```

`labels` defaults to `["wine glass", "cup"]` (COCO classes) and `min_confidence` to `0.5`. The camera must be in the frame system.

## CLI

```bash
make cli            # builds bin/cocktail-cli
viam login          # the CLI uses cached viam credentials
```

### hover-glass

Finds the highest-confidence glass and moves `--component` (tool facing down) to its world-frame (x, y) centroid at `--hover-z-mm` (default 300 mm). The glass finder is built locally from the machine's resources, so the module does not need to be deployed.

```bash
# Print the predicted glass position and target pose without moving; save the glass cloud for inspection.
bin/cocktail-cli hover-glass --machine-address <part>.viam.cloud --camera cam --dry-run --save-pcd glass.pcd

# Hover above the glass.
bin/cocktail-cli hover-glass --machine-address <part>.viam.cloud --camera cam --component gripper
```

### pour-glass

Finds the glass with the wrist camera, picks up `--bottle`, pours `--pour-ms` into the glass, and puts the bottle back (bartender DoCommand `find_and_pour`). `--find-only` stops after finding the glass (`find_glass`): the arm moves to search but never touches the bottle. Move the glass between runs to check the (x, y) at many positions.

Finding the glass:
1. Move to the saved `glass-look` pose (same pose switcher as `home`): where the camera looks from, not where the glass is.
2. Aim point = where the camera's optical axis meets the table (`glass_table_z_mm`, default 0).
3. If no glass is visible, lower the camera a few millimetres at a time (`glass_search_lower_mm`, default `[0, 5, 10]`), keeping x, y and re-aiming at the same point, waiting `glass_search_settle_ms` (1000) after each move. The first view with a glass wins; unreachable views are skipped.

Pouring: the saved `pour-approach` and `pour-tilt` poses are translated in world x, y from `pour_reference_glass` to the found glass, keeping the saved height and tilt. Glasses farther than `max_pour_offset_mm` (300) from the reference are rejected before the bottle is picked up.

Bartender config:
```json
"glass_finder_name": "glass-finder",
"pour_reference_glass": {"x": 520, "y": -80},
"max_pour_offset_mm": 300
```
`glass_finder_name` is a `viam:cocktail-bot:glass-finder` vision service on the wrist camera.

```bash
# Calibrate once: place a glass where the saved pour lands, find it, and copy its x, y into pour_reference_glass.
bin/cocktail-cli pour-glass --machine-address <part>.viam.cloud --find-only

bin/cocktail-cli pour-glass --machine-address <part>.viam.cloud --bottle bottle-gin --pour-ms 1500
```
