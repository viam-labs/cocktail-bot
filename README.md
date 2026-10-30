# cocktail-bot

Viam module that drives a UFactory xArm to make cocktails end-to-end: ice → pour → mix → strain → serve → rotate shakers.

## Models

- [`viam:cocktail-bot:bartender`](bartender/README.md) (`rdk:service:generic`) — orchestrates the full drink cycle. All config and DoCommands documented here.
- `viam:cocktail-bot:pose-switcher` (`rdk:component:switch`) — holds named poses (`hover`, `grab`, `carry-hover`, `pour-approach`, etc.) that the bartender resolves at runtime. Supports absolute poses and baseline-relative deltas with optional per-pose velocity/acceleration.
- `viam:cocktail-bot:glass-finder` (`rdk:service:vision`) — locates glasses in 3D. Runs a 2D detector on the camera image, keeps the camera point cloud points that project inside each glass box, and returns them in the world frame.
- `viam:cocktail-bot:order-sensor` (`rdk:component:sensor`) — one reading per order attempt, for Data Management.
- [`viam:cocktail-bot:inventory-sensor`](inventorysensor/README.md) (`rdk:component:sensor`) — snapshots bartender inventory and slot assignments on change and hourly, for Data Management.
- `viam:cocktail-bot:maintenance-sensor` (`rdk:component:sensor`) — reports whether the bartender is idle, so Viam's maintenance-allowed gate can block reconfigures mid-cocktail.

### `glass-finder` configuration

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

`labels` defaults to `["wine glass", "cup"]` (COCO classes); `min_confidence` to `0.5`. The camera must be in the frame system.

## Other components

- [`cmd/cli`](cmd/cli/README.md) — `cocktail-cli`, developer CLI for vision + motion debugging.
- `web-app/` — Next.js kiosk + fleet dashboard. Packaged as its own Viam module via `make web-app-module`.
