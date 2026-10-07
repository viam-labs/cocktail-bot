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

`viam:cocktail-bot:spill-glass-finder` (`rdk:service:vision`) is a separate Python module in [`spill-glass-finder/`](spill-glass-finder/README.md). It fits each transparent glass's rim center, radius and height from SPILL keypoints and the table plane.

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

With `--glass-finder <name>`, the CLI uses a deployed `spill-glass-finder` vision service instead of the local glass finder. It calls `DoCommand({"find_glasses": {}})` and transforms the highest-scoring glass's camera-frame rim center to the world frame. It logs radius, height and tilt, then hovers at the world (x, y) of the rim center. `--camera`, `--detector`, `--labels` and `--min-confidence` are ignored, and `--save-pcd` is rejected.

```bash
bin/cocktail-cli hover-glass --machine-address <part>.viam.cloud --glass-finder spill-glass-finder --dry-run
```
