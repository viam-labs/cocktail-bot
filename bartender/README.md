# Bartender

Orchestrates the full drink cycle: ice → pour → mix → strain → serve → rotate shakers.

## Model `viam:cocktail-bot:bartender`

### Configuration for bartender model

#### Attributes

| Name | Type | Inclusion | Description | Default |
| --- | --- | --- | --- | --- |
| `arm_name` | string | Required | Arm the service drives. |  |
| `gripper_name` | string | Required | Gripper on the arm. |  |
| `pose_switcher_names` | []string | Required | Every pose-switcher component the bartender resolves poses on. |  |
| `held_bottle_geometry` | object | Optional | Cylinder attached to the gripper while a bottle is held. See below. |  |
| `held_shaker_geometry` | object | Optional | Cylinder attached to the gripper while the shaker is held. See below. |  |
| `pickup_obstacles` | object | Optional | Map of pickup key → obstacle name; the gripper and held frame may intersect that obstacle during pickup/release of that key. Keys are bottle switch names plus `shaker`, `shaker-a`, `shaker-b`. |  |
| `bottle_pourer_oz` | object | Optional | Map of bottle switch name → oz delivered per pour-tilt. Each recipe pour oz must be an integer multiple of this. |  |
| `recipe_defaults` | object | Optional | Default stations and dwell times for `make_cocktail`. See below. |  |
| `serving_station` | string | Optional | Switch holding the shaker and serve poses used by `find_and_pour_from_shaker`. | `serving-glass-center` |
| `pour_mouth_offset_mm` | number | Optional | Horizontal gripper-to-bottle-mouth distance while pouring, on the side the bottle tips. Negative flips the side. | 100 |
| `max_pour_offset_mm` | number | Optional | Max world-frame shift of `serve-approach`/`serve-tilt` from their saved positions; glasses beyond fail before pickup. | 300 |
| `pour_into_shaker_dwell_ms` | int | Optional | Dwell at `pour-tilt` on each pour into the shaker. | 500 |
| `pour_vel_degs_per_sec` | number | Optional | Angular velocity on pour-tilt moves. Arm default otherwise. |  |
| `pour_acc_degs_per_sec2` | number | Optional | Angular acceleration on pour-tilt moves. |  |
| `serve_vel_degs_per_sec` | number | Optional | Angular velocity on serve-tilt moves. When unset, falls back to per-pose vel on `serve-tilt`. |  |
| `serve_acc_degs_per_sec2` | number | Optional | Angular acceleration on serve-tilt moves. |  |
| `mixer_protector_obstacle` | string | Optional | Obstacle name whose collisions are allowed during `mix()` only. |  |
| `mixer_power_switch_name` | string | Optional | Switch that powers the mixer, e.g. `viam:zigbee2mqtt:switch` on a smart plug. `mix()` turns it on once the shaker is seated and off before withdrawing (and on error, cancel, or close). When set, the switch is a required dependency. If switching fails, the order continues and a Slack warning asks for the mixer to be switched by hand. Empty leaves the mixer manual. |  |
| `bottles_protector_obstacle` | string | Optional | Obstacle name whose collisions are allowed during `pickupBottle` and `returnBottle` only. |  |
| `glass_finder_name` | string | Optional | Vision service returning world-frame glass point clouds, e.g. `viam:cocktail-bot:glass-finder` on the wrist camera. |  |
| `glass_table_z_mm` | number | Optional | World z of the table surface; the search aims the camera where its `glass-look` view meets it. | 0 |
| `glass_search_lower_mm` | []number | Optional | Mm below `glass-look` to lower the camera between search attempts. | `[0, 5, 10]` |
| `glass_search_pan_deg` | []number | Optional | Degrees to pan at each height; positive = left. | `[0, 5, -5]` |
| `glass_search_settle_ms` | int | Optional | Settle time after each search move before capture. | 1000 |
| `glass_min_separation_mm` | number | Optional | Detections closer than this in x, y count as one glass. | 50 |
| `order_sensor_name` | string | Optional | Order sensor to push each attempt reading to. |  |
| `data_dir` | string | Optional | Directory for persisted recipes + inventory. Falls back to `$VIAM_MODULE_DATA`. |  |
| `save_motion_requests_dir` | string | Optional | When set, every motion-plan request + response is written here, tagged by order id. |  |
| `slack_webhook_url` | string | Optional | Incoming webhook URL for order outcome alerts. |  |
| `machine_id` | string | Optional | Viam machine id; when set, failure alerts deep-link to app.viam.com machine logs. |  |

##### `held_bottle_geometry` and `held_shaker_geometry`

Capsule attached as a child of the gripper frame while the object is held.

| Name | Type | Inclusion | Description | Default |
| --- | --- | --- | --- | --- |
| `type` | string | Required | Must be `cylinder`. |  |
| `radius_mm` | number | Required | Cylinder radius. |  |
| `length_mm` | number | Required | Cylinder length. |  |
| `z_offset_mm` | number | Optional | Offset in gripper +z. | 0 |

##### `recipe_defaults`

Stations and dwell times `make_cocktail` passes through to the primitives.

| Name | Type | Inclusion | Description | Default |
| --- | --- | --- | --- | --- |
| `shaker_source` | string | Required | Switch holding shaker-home poses. |  |
| `ice_station` | string | Required | Switch for the ice lever. |  |
| `mixer_station` | string | Required | Switch holding mixer poses. |  |
| `strain_flow` | string | Required | Switch holding strain + shaker-a/b poses. |  |
| `serve_station` | string | Required | Default `serving_station` for the serve step. |  |
| `ice_dwell_ms` | int | Optional | Dwell at the ice lever. | 0 |
| `mix_dwell_ms` | int | Optional | Mixing bob duration. | 0 |
| `serve_ms` | int | Optional | Pour duration into the glass. | 0 |
| `strain_drain_ms` | int | Optional | Drain dwell during straining. | 0 |
| `strain_dump_ms` | int | Optional | Dump dwell over garbage. | 0 |

#### Example Configuration

```json
{
  "name": "bartender",
  "api": "rdk:service:generic",
  "model": "viam:cocktail-bot:bartender",
  "attributes": {
    "arm_name": "arm-1",
    "gripper_name": "gripper-1",
    "pose_switcher_names": ["coffee-liquor", "vodka", "stations", "mixer", "ice-station", "strain-flow", "serving-glass-center"],
    "held_bottle_geometry": {"type": "cylinder", "radius_mm": 35, "length_mm": 550},
    "held_shaker_geometry": {"type": "cylinder", "radius_mm": 45, "length_mm": 170, "z_offset_mm": 184},
    "bottle_pourer_oz": {"vodka": 1.5, "coffee-liquor": 1},
    "pickup_obstacles": {
      "vodka": "vodka-bottle",
      "coffee-liquor": "coffee-liquor-bottle",
      "shaker-a": "shaker-a-obstacle",
      "shaker-b": "shaker-b-obstacle"
    },
    "recipe_defaults": {
      "shaker_source": "ice-station",
      "ice_station": "ice-station",
      "mixer_station": "mixer",
      "strain_flow": "strain-flow",
      "serve_station": "serving-glass-center",
      "ice_dwell_ms": 3000,
      "mix_dwell_ms": 10000,
      "serve_ms": 5000
    },
    "mixer_protector_obstacle": "mixer-protector-obstacle",
    "bottles_protector_obstacle": "bottles-protector-obstacle"
  }
}
```

### DoCommand

#### Cocktail flow

##### `make_cocktail`

Runs a full recipe end-to-end. Async: returns `{started_at}` immediately; watch progress via `get_status`.

```json
{"make_cocktail": {"drink_id": "espresso_martini"}}
```

Pass `"recipe"` instead of `"drink_id"` to run an unsaved recipe:

```json
{"make_cocktail": {"recipe": {"id": "ad_hoc", "name": "Ad hoc", "pours": [{"ingredient": "vodka", "oz": 1.5}]}}}
```

##### `prepare_order`

Enqueue an order on the queue.

##### `cancel`

Cancel the running order.

##### `get_status`

Snapshot of the running or last order. Returns `{is_busy, current_step, drink, started_at, elapsed_ms, step_history, last_outcome: {drink, ended_at, error?}}`.

##### `get_queue`

Current queue state.

#### Primitives

| Command | Shape |
| --- | --- |
| `dispense_ice` | `{"station": "ice-station", "dwell_ms": 3000}` |
| `pour_into_shaker` | `{"bottle": "vodka", "oz": 1.5}` |
| `mix` | `{"station": "mixer", "dwell_ms": 10000}` |
| `strain_shaker` | `{"source": "ice-station", "strain_flow": "strain-flow", "drain_dwell_ms": 4000, "dump_dwell_ms": 2000}` |
| `pour_from_shaker` | `{"station": "serving-glass-center", "pour_ms": 5000}` |
| `rotate_shakers` | `{"strain_flow": "strain-flow"}` |
| `pickup_pour_return` | `{"bottle": "vodka", "pour_ms": 1500}` |
| `pour_into_glasses` | `{"bottle": "vodka", "pours": [{"glass": "serving-glass-center", "pour_ms": 1500}]}` |
| `execute_action` | `{"pose_name": "home"}` — move to any named pose on the configured switchers. |

#### Vision-driven

##### `find_glass`

Run the glass search; return `{glass: {label, x, y, lower_mm, pan_deg}}`.

##### `find_and_pour`

Find the glass, then run `pickup_pour_return` with `pour-approach`/`pour-tilt` shifted to the glass.

```json
{"find_and_pour": {"bottle": "vodka", "pour_ms": 1500, "mouth_offset_mm": 80}}
```

`mouth_offset_mm` is optional; omit to use `pour_mouth_offset_mm`.

##### `find_and_pour_from_shaker`

Serve the shaker's contents into every glass found. Glasses farther than `max_pour_offset_mm` from the saved serve-tilt are skipped.

```json
{"find_and_pour_from_shaker": {"pour_ms": 5000}}
```

#### Recipes and inventory

| Command | Shape |
| --- | --- |
| `get_recipes` | `{}` → `{recipes: [...]}` |
| `update_recipes` | `{"recipes": [{"id": "...", "name": "...", "image": "...", "on_menu": true, "pours": [{"ingredient": "...", "oz": N}]}]}` |
| `get_inventory` | `{}` → `{ingredients: {name: {in_stock: bool}}}` |
| `update_inventory_item` | `{"ingredient": "vodka", "in_stock": true}` |
| `delete_inventory_item` | `{"ingredient": "vodka"}` |
| `get_bottles` | `{}` → `{bottles: [...]}` — switch names in `pose_switcher_names` that are also in `bottle_pourer_oz`. |
