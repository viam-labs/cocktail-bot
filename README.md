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
3. If no glass is visible, try nearby views. At each height (`glass_search_lower_mm`, default `[0, 5, 10]` mm lower, x, y kept and re-aimed at the same point) the camera also pans in place (`glass_search_pan_deg`, default `[0, 5, -5]`; positive = left), waiting `glass_search_settle_ms` (1000) after each move. The first view with a glass wins; unreachable views are skipped.

Pouring: the bottle mouth is assumed to sit `pour_mouth_offset_mm` (default 100) from the gripper, toward the side the top of the bottle tips when the gripper rotates from the saved `pour-approach` to `pour-tilt`. Both saved poses are translated in world x, y so the mouth ends up above the glass, keeping the saved height and tilt. A negative offset flips the side. If the pour would move more than `max_pour_offset_mm` (300) from the saved `pour-tilt`, the command fails before the bottle is picked up.

Bartender config:
```json
"glass_finder_name": "glass-finder",
"pour_mouth_offset_mm": 100,
"max_pour_offset_mm": 300
```
`glass_finder_name` is a `viam:cocktail-bot:glass-finder` vision service on the wrist camera.

```bash
# Find the glass only; the bottle is never touched.
bin/cocktail-cli pour-glass --machine-address <part>.viam.cloud --find-only

bin/cocktail-cli pour-glass --machine-address <part>.viam.cloud --bottle bottle-gin --pour-ms 1500

# Try a different mouth offset for this run only.
bin/cocktail-cli pour-glass --machine-address <part>.viam.cloud --bottle bottle-gin --pour-ms 1500 --mouth-offset-mm 80
```
