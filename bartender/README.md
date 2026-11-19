# Bartender

Orchestrates the full drink cycle: ice → pour → mix → strain → serve → clean shaker (optional) → rotate shakers.

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
| `held_stirrer_geometry` | object | Optional | Cylinder attached to the gripper while the stirrer is held, from gripper close to gripper open in `stir`. See below. |  |
| `pickup_obstacles` | object | Optional | Map of pickup key → obstacle name; the gripper and held frame may intersect that obstacle during pickup/release of that key. Keys are bottle switch names plus `shaker`, `shaker-a`, `shaker-b`, `stirrer`. `stir` also uses the `shaker` entry while the stirrer is inside the shaker. |  |
| `recipe_defaults` | object | Optional | Default stations and dwell times for `make_cocktail`. See below. |  |
| `glasses` | []object | Optional | Glass types staff can place at the serving station. Recipes pick one by `id`. See below. |  |
| `default_glass_obstacle` | string | Optional | Obstacle modeling the glass at the station for recipes with no glass, glasses with no `obstacle`, and before the first order. See `glasses`. |  |
| `serving_station` | string | Optional | Switch holding the shaker and serve poses used by `find_and_pour_from_shaker`. | `serving-glass-center` |
| `pour_mouth_offset_mm` | number | Optional | Horizontal gripper-to-bottle-mouth distance while pouring, on the side the bottle tips. Negative flips the side. | 100 |
| `max_pour_offset_mm` | number | Optional | Max world-frame shift of `serve-approach`/`serve-tilt` from their saved positions; glasses beyond fail before pickup. | 300 |
| `pour_into_shaker_dwell_ms` | int | Optional | Dwell at the tilt on each pour from a bottle, into the shaker or a glass, when the bottle's pourer size is not in `pour_dwell_ms_by_pourer_oz`. | 500 |
| `pour_dwell_ms_by_pourer_oz` | object | Optional | Map of pourer size in oz (string key, e.g. `"1.5"`) → dwell ms at the tilt on each pour from a bottle with that pourer. Pourer sizes are set per ingredient in the inventory. Example: `{"0.5": 1500, "1": 2500, "1.5": 3500}`. |  |
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
| `loading_spot_switch_name` | string | Optional | Pose-switcher component holding the loading-spot `hover`/`grab`/`lift`/`carry-hover` poses used by `stock_bottle`, `retrieve_bottle`, and `move_bottle`. Required for those commands. |  |
| `data_dir` | string | Optional | Directory for persisted recipes, inventory, the order queue (`order_queue.json`) and whether the fridge door is open (`fridge_door.json`). Falls back to `$VIAM_MODULE_DATA`. With neither set, the queue is kept in memory only. |  |
| `save_motion_requests_dir` | string | Optional | When set, every motion-plan request + response is written here, tagged by order id. |  |
| `slack_webhook_url` | string | Optional | Incoming webhook URL for order outcome alerts. |  |
| `machine_id` | string | Optional | Viam machine id; when set, failure alerts deep-link to app.viam.com machine logs. |  |
| `shaker_cleaning` | object | Optional | v1 feature flag: rinse the shaker on the cleaner after `make_cocktail` serves it. See below. | off |
| `fridge` | object | Optional | Fridge door the arm swings open by its handle. Needs `data_dir` or `$VIAM_MODULE_DATA` to remember whether the door is open. See below. |  |

##### `held_bottle_geometry`, `held_shaker_geometry` and `held_stirrer_geometry`

Capsule attached as a child of the gripper frame while the object is held.

| Name | Type | Inclusion | Description | Default |
| --- | --- | --- | --- | --- |
| `type` | string | Required | Must be `cylinder`. |  |
| `radius_mm` | number | Required | Cylinder radius. |  |
| `length_mm` | number | Required | Cylinder length. |  |
| `z_offset_mm` | number | Optional | Offset in gripper +z. | 0 |

##### `glasses`

Each glass names the pose switcher holding its taught poses, the same way each bottle slot does.

| Name | Type | Inclusion | Description | Default |
| --- | --- | --- | --- | --- |
| `id` | string | Required | What recipes store in `glass` and `double_glass`. |  |
| `name` | string | Required | Shown to staff, e.g. "Highball". |  |
| `capacity_oz` | number | Required | How much the glass holds. `make_cocktail` refuses a drink that needs more, counting melt water from mixing over ice and the shaker's ice when the drink isn't strained. |  |
| `image` | string | Optional | Outline the web app draws: `highball`, `martini`, `rocks`, `coupe` or `wine` (stemless). | `rocks` |
| `pose_switcher` | string | Required | Switch in `pose_switcher_names` holding this glass's `serve-approach`, optional `serve-pre-tilt` and `serve-tilt` (used by drinks made in the shaker) and `direct-pour-approach`, `direct-pour-tilt` and `direct-pour-recover` (used by drinks poured straight from the bottle). |  |
| `obstacle` | string | Optional | Obstacle modeling this glass, a component such as `erh:vmodutils:obstacle`. The gripper and held shaker or bottle may intersect it while pouring into the glass. | `default_glass_obstacle` |

A serve moves the shaker from `serve-approach` to `serve-tilt` at the serve speed (`serve_vel_degs_per_sec`, else `serve-tilt`'s own), since that tilt is the pour. Add an optional `serve-pre-tilt` pose on the same switch, tilted to just before liquid reaches the shaker lip, and the shaker gets there at `serve-pre-tilt`'s own speed (the arm's default when unset) and only tilts the rest of the way slowly.

```json
"glasses": [
  {"id": "martini", "name": "Martini", "capacity_oz": 7, "image": "martini", "pose_switcher": "serve-martini", "obstacle": "martini-glass"},
  {"id": "highball", "name": "Highball", "capacity_oz": 12, "image": "highball", "pose_switcher": "serve-highball", "obstacle": "highball-glass"}
]
```

A recipe with no glass serves at `recipe_defaults.serve_station`, and its direct pours use the first pose switcher that defines the `direct-pour-*` poses.

Only one glass is at the station at a time, so motion planning includes only one glass obstacle: the glass of the order being made, or of the last order made, since that glass stays until someone takes it. Every other glass obstacle and `default_glass_obstacle` is left out, along with any `allowed_obstacles` entry that names one. A recipe with no glass, and every move before the first order, uses `default_glass_obstacle`, as does a glass with no `obstacle`. Glass obstacles are dependencies, so the service won't start if one is missing.

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
| `stir_station` | string | Optional | Switch holding `stir-storage-hover` / `stir-storage-place` / `stir-hover` / `stir-insert`, used by `mix` steps with `"mode": "stir"`. Teach `stir-hover` as `joint_positions_degs` and give `stir-insert` its own `pose_value`: a `stir-hover` given as a pose lets the planner arrive with the wrist flipped and J6 near its limit, and the straight insert then fails with "no direct solutions found". | `stir` |
| `stir_dwell_ms` | int | Optional | How long a `stir` step circles. | `mix_dwell_ms` |
| `serve_ms` | int | Optional | Pour duration into the glass. | 0 |
| `strain_drain_ms` | int | Optional | Drain dwell during straining. | 0 |
| `strain_dump_ms` | int | Optional | Dump dwell over garbage. | 0 |

##### `shaker_cleaning`

When enabled, `make_cocktail` puts the shaker back at shaker-home after serving, then picks it up again with a lower grip so it can be flipped, rinses it on the cleaner and sets it back down at shaker-home. The cleaner sprays water while the upside-down shaker is pressed onto it.

| Name | Type | Inclusion | Description | Default |
| --- | --- | --- | --- | --- |
| `enabled` | bool | Optional | Turns the cleaning step on. | false |
| `station` | string | Required when enabled | Pose switcher holding the cleaner poses below. Must be in `pose_switcher_names`. |  |
| `dwell_ms` | int | Optional | How long the shaker stays pressed on the cleaner. | 0 |
| `obstacle` | string | Optional | Obstacle name modeling the cleaner; the gripper, arm and held shaker may intersect it during the cleaning moves. |  |
| `power_switch_name` | string | Optional | Switch that powers the cleaner (e.g. a smart plug). When set, it turns on once the shaker is pressed onto the cleaner and off before the retreat. A failure to toggle fails the order and posts a Slack alert. When unset, the cleaner is expected to spray mechanically when pressed. |  |
| `power_on_delay_ms` | int | Optional | Pause after pressing the shaker onto the cleaner before turning the cleaner on. | 0 |
| `drain_ms` | int | Optional | Pause after cutting cleaner power before lifting the shaker off, so water can drip off. | 0 |
| `prestage_pose` | string | Optional | Joint-space pose on `station` the empty arm passes through before the cleaner grab, like `mix_prestage_pose`. |  |
| `held_shaker_geometry` | object | Optional | Cylinder attached to the gripper while the cleaner's lower grip holds the shaker. Same shape as `held_shaker_geometry`. | `held_shaker_geometry` |

The `station` switch needs these poses:

| Pose | Shaker |
| --- | --- |
| `cleaner-grab-hover`, `cleaner-grab-approach`, `cleaner-grab-lift`, `cleaner-grab-carry-hover` | The lower grip on the shaker at shaker-home, used the same way as the `shaker-home-*` poses to pick it up and set it back down. |
| `cleaner-hover` | Upright, above the cleaner. |
| `cleaner-flip-hover` | Upside down, same spot as `cleaner-hover`. |
| `cleaner-approach` | Upside down, mouth just above the cleaner. Also where the shaker backs off to so the spray stops. |
| `cleaner-press` | Upside down, pressed onto the cleaner so it sprays. |

```json
"shaker_cleaning": {"enabled": true, "station": "shaker-cleaner", "dwell_ms": 3000, "obstacle": "cleaner-obstacle", "prestage_pose": "cleaner-prestage"}
```

##### `fridge`

The arm opens the fridge by gripping the door handle and swinging it about the hinge, then leaves the door standing open until it closes it. While the door is open, every plan swings `door_obstacle` about the hinge to the door's angle, so moves avoid the open door.

| Name | Type | Inclusion | Description | Default |
| --- | --- | --- | --- | --- |
| `door_switch_name` | string | Required | Pose switcher holding the handle poses below. Must be in `pose_switcher_names`. |  |
| `door_obstacle` | string | Required | Obstacle modeling the door panel shut, parented to world. The gripper may touch it while grabbing, swinging and letting go of the handle. |  |
| `hinge` | object | Required | `{"x", "y"}`: world position (mm) of the vertical hinge axis. |  |
| `swing_step_degs` | number | Optional | Largest door angle per planned step of the swing. | 10 |
| `swing_vel_degs_per_sec` | number | Optional | Angular velocity of the swing. Arm default otherwise. |  |
| `swing_acc_degs_per_sec2` | number | Optional | Angular acceleration of the swing. |  |
| `slots` | []string | Optional | Bottle slots inside the fridge. Each must be in `slot_names`. |  |

The `door_switch_name` switch needs these poses:

| Pose | Door | Used for |
| --- | --- | --- |
| `closed-hover` | Shut | Above the handle before grabbing it to open. |
| `closed-grab` | Shut | Gripper closes on the handle to open. Where closing ends. |
| `open-retreat` | Open | Backed off from the handle after letting go of the open door. |
| `open-hover` | Open | Above the handle before grabbing it to close. |
| `open-grab` | Open | Where opening ends. Gripper closes on the handle to close. |
| `closed-retreat` | Shut | Backed off from the handle after letting go of the shut door. |

The swing angle is the angle between `closed-grab` and `open-grab` about the hinge. The gripper holds the handle rigidly, so `open-grab` must be `closed-grab` turned about the hinge: within 15 mm and 10°, or the swing errors before moving. Each step is a straight line within 9 mm and 9°, and all steps run as one arm move.

The door is recorded open before an opening swing starts moving and recorded shut only after a closing swing finishes. While it is recorded open outside a swing, for example after a failed swing, `make_cocktail` is refused. Shut the door by hand and press `cancel` with nothing running to record it shut. A `cancel` that stops a running command leaves the record alone, since the door may be partway open.

Fridge slots take the same `hover`, `grab`, `lift` and `carry-hover` poses as shelf slots; list the fridge cabinet in their `allowed_obstacles` where the arm reaches inside. The fridge has one row, so a fridge slot's `hover` must not set `blocked_by`. Picking up a bottle from a fridge slot, for any pour, opens the door first, and putting it back closes the door again. If the door already stood open, it is left open. `grab_bottle` on a fridge slot leaves the door open, since the bottle is never put back. `stock_bottle`, `retrieve_bottle` and `move_bottle` refuse fridge slots for now.

```json
"fridge": {"door_switch_name": "fridge-door-handle", "door_obstacle": "fridge-door", "hinge": {"x": 350, "y": -900}, "swing_vel_degs_per_sec": 20, "slots": ["slot-a-fridge-front", "slot-b-fridge-front"]}
```

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
    "pour_dwell_ms_by_pourer_oz": {"0.5": 1500, "1": 2500, "1.5": 3500},
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

Adds a full recipe to the order queue and returns immediately with `{order_id, title, glass, ahead, enqueued_at, ...}`, where `ahead` is how many orders are in front of it and `glass` is the name of the glass staff should place: the recipe's `double_glass` for a double, otherwise its `glass`, or empty if the recipe names none. The drink is poured using that glass's poses. Fails if the glass isn't in `glasses`, if the drink won't fit in it, or if a double is ordered for a recipe that names a `glass` but no `double_glass`. Watch progress via `get_queue`.

The queue is saved to `order_queue.json` in `data_dir` on every change, so waiting orders survive a restart or reconfigure. An order is rejected if it can't be saved. If the module stops while a drink is being made, that drink isn't remade on restart: it's recorded as failed ("interrupted"), reported to the order sensor and Slack, and the queue holds until `start_next`. A corrupt queue file stops the service from starting, with an error naming the file.

The queue makes one drink at a time. Every drink is served at the same station, so after each order (succeeded, failed or cancelled) the queue holds until `start_next` confirms the drink was taken and a fresh glass is in place. An order that names a glass also waits for `start_next` unless it was next in line when `start_next` was called, since only then were staff shown which glass to place: for example, the first order after the line was empty, or after a restart.

```json
{"make_cocktail": {"drink_id": "espresso_martini"}}
```

With `shaker_cleaning` enabled, every drink served from the shaker returns the shaker home and then runs `clean_shaker` at `shaker_cleaning.station`, shown as the `Cleaning shaker` step. The order finishes once the cleaned shaker is back home.

A `mix` step with `"mode": "stir"` runs `stir` at `stir_station` instead of shaking. A recipe that stirs parks at `home` afterwards without picking up the shaker or serving.

Pass `"recipe"` instead of `"drink_id"` to run an unsaved recipe:

```json
{"make_cocktail": {"recipe": {"id": "ad_hoc", "name": "Ad hoc", "pours": [{"ingredient": "vodka", "oz": 1.5}]}}}
```

##### `get_queue`

`{is_busy, count, current?, awaiting_pickup?, awaiting_glass?, queue: [...], recent: [...]}`. Each order is `{id, drink, double, quantity, glass?, enqueued_at, started_at, raw_step, completed_at, error?}`. `awaiting_pickup` is the finished order holding the queue; `awaiting_glass` is the next order, held until someone confirms its glass is at the station; `recent` keeps finished orders for two minutes so clients can look up their outcome by `id`.

##### `start_next`

Confirm the station is ready: clear the pickup hold, and confirm the next order's glass is in place, so that order starts. Returns `{released}`.

##### `cancel_order`

Remove an order that hasn't started: `{"cancel_order": {"order_id": "..."}}`. Returns `{removed}`.

##### `cancel`

Cancel the running order. Pending orders stay queued behind the pickup hold. With nothing running, it also records the fridge door shut; see `fridge`.

##### `get_status`

Snapshot of the running or last order. Returns `{is_busy, count, current_step, drink, started_at, elapsed_ms, step_history, last_outcome: {drink, ended_at, error?}}`, plus `fridge_door_open_degs` (0 when shut) when `fridge` is configured.

Primitives, vision-driven commands, `strain_shaker`, `rotate_shakers`, `execute_action`, `stock_bottle`, `retrieve_bottle` and `move_bottle` return an error while a queued drink is being made rather than interrupting it.

#### Primitives

| Command | Shape |
| --- | --- |
| `dispense_ice` | `{"station": "ice-station", "dwell_ms": 3000}` |
| `pour_into_shaker` | `{"bottle": "vodka", "oz": 1.5}` |
| `mix` | `{"station": "mixer", "dwell_ms": 10000}` |
| `stir` | `{"station": "stir-station", "dwell_ms": 20000}` — `stir-storage-hover` → open → linear down to `stir-storage-place` → close (stirrer held) → linear up to `stir-storage-hover` → `stir-hover` → linear down to `stir-insert` → circles around `stir-insert` for `dwell_ms` → linear back to `stir-hover` → `stir-storage-hover` → linear down to `stir-storage-place` → open (stirrer released) → linear up to `stir-storage-hover`. `stir-storage-hover` must be high enough that the held stirrer clears its storage. When `stir-hover` is a joint-space pose the carry ends at exactly those joints, and the retract goes straight up to where those joints put the arm. Circle size comes from `circle_radius_mm` (default 10) and `circle_points_per_rev` (default 8, min 4) on the `stir-insert` pose. |
| `strain_shaker` | `{"source": "ice-station", "strain_flow": "strain-flow", "drain_dwell_ms": 4000, "dump_dwell_ms": 2000}` |
| `pour_from_shaker` | `{"station": "serving-glass-center", "pour_ms": 5000}` |
| `clean_shaker` | `{"station": "shaker-cleaner", "dwell_ms": 3000, "power_on_delay_ms": 500, "drain_ms": 1000}` — gripper must be empty with the shaker at shaker-home. Optional `shaker_cleaning.prestage_pose` joint move → pick the shaker up with the `cleaner-grab-*` poses → carry level to `cleaner-hover` → flip to `cleaner-flip-hover` → linear down to `cleaner-approach` → linear down to `cleaner-press` → wait `power_on_delay_ms` → cleaner on → wait `dwell_ms` → cleaner off → wait `drain_ms` for drips → linear up to `cleaner-approach` → linear up to `cleaner-flip-hover` → flip upright at `cleaner-hover` → set the shaker back down at shaker-home with the `cleaner-grab-*` poses. The cleaner on/off steps only happen when `shaker_cleaning.power_switch_name` is set. `power_on_delay_ms` and `drain_ms` default to 0 and are optional. Runs regardless of `shaker_cleaning.enabled`, so poses can be tuned before turning it on. |
| `open_fridge_door` | `{}` — gripper must be empty and the door recorded shut. `closed-hover` → open gripper → linear to `closed-grab` → close gripper → swing to `open-grab` → open gripper → linear to `open-retreat`. Leaves the door open. Returns `{open_degs, duration_ms}`. |
| `close_fridge_door` | `{}` — gripper must be empty and the door recorded open. `open-hover` → open gripper → linear to `open-grab` → close gripper → swing to `closed-grab` → open gripper → linear to `closed-retreat`. Returns `{open_degs, duration_ms}`. |
| `rotate_shakers` | `{"strain_flow": "strain-flow"}` |
| `pickup_pour_return` | `{"bottle": "vodka", "pour_ms": 1500}` |
| `pour_into_glasses` | `{"bottle": "vodka", "pours": [{"glass": "serving-glass-center", "pour_ms": 1500}]}` |
| `execute_action` | `{"pose_name": "home"}` — move to any named pose on the configured switchers. |

#### Stocking

Move bottles between the shelf and the loading spot. The arm uses the existing blocker-shuffle flow if the target slot is behind an occupied blocker. The inventory data store updates only after the arm reports success. Fridge slots are refused; move those bottles by hand.

| Command | Shape |
| --- | --- |
| `stock_bottle` | `{"slot": "slot-g-bottom-front", "ingredient": "aged-rum"}` — pick up a bottle sitting at `loading_spot_switch_name`, place it in `slot`, assign `ingredient` to `slot`. |
| `retrieve_bottle` | `{"slot": "slot-g-bottom-front"}` — pick the bottle out of `slot`, leave it on the loading spot, clear `slot`'s assignment. |
| `move_bottle` | `{"from": "slot-a-bottom-front", "to": "slot-g-bottom-front"}` — carry `from`'s bottle to `to`. `to` must be empty. Fails if both slots are blocked by different occupied blockers (there's only one parking spot). |

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
| `update_recipes` | `{"recipes": [{"id": "...", "name": "...", "image": "...", "on_menu": true, "glass": "martini", "double_glass": "highball", "pours": [{"ingredient": "...", "oz": N}]}]}` — `glass` and `double_glass` must be ids from the `glasses` config, and a recipe that names a `glass` and can be doubled must also name a `double_glass`. |
| `get_glasses` | `{}` → `{glasses: [...]}`, the `glasses` config. |
| `get_inventory` | `{}` → `{ingredients: {name: {in_stock: bool}}}` |
| `update_inventory_item` | `{"ingredient": "vodka", "in_stock": true}` |
| `delete_inventory_item` | `{"ingredient": "vodka"}` |
| `get_bottles` | `{}` → `{bottles: [...]}` — ingredients currently assigned to a slot. |
