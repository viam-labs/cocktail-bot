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

### pour-glasses

Moves to `--look-pose` (default `home`), detects every glass with the glass finder, and asks the bartender to pick up `--bottle` once and pour `--pour-ms` into each glass. Detections closer than `--min-separation-mm` (50) are treated as one glass.

The bartender pours by translating its saved `pour-approach` and `pour-tilt` poses in world x, y from `pour_reference_glass` to each detected glass, so every pour keeps the saved height and tilt. Glasses farther than `max_pour_offset_mm` (default 300) from the reference are rejected before the arm moves.

```json
"pour_reference_glass": {"x": 520, "y": -80},
"max_pour_offset_mm": 300
```

```bash
# Calibrate once: place one glass where the saved pour lands, detect, and copy its x, y into pour_reference_glass.
bin/cocktail-cli pour-glasses --machine-address <part>.viam.cloud --camera cam --dry-run

# Pour into every glass in view.
bin/cocktail-cli pour-glasses --machine-address <part>.viam.cloud --camera cam --bottle bottle-gin --pour-ms 1500
```

The camera is on the wrist, so `--dry-run` detects from wherever the arm is; move it to the look pose first.
