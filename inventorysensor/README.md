# `viam:cocktail-bot:inventory-sensor`

`rdk:component:sensor` that snapshots the bartender's inventory (bottles in stock, bottles in each slot, pourer oz, categories, whiskey profiles) so Data Management can sync it to Viam as a backup and history. The bartender's `data_dir` stays the source of truth; nothing is restored from Viam.

Each `Readings` call fetches `get_inventory` from the bartender. It returns the snapshot when it differs from the last one returned or when `heartbeat_sec` has passed since then; otherwise it returns `ErrNoCaptureToStore` so capture skips it. Pair a frequent capture rate with the hourly heartbeat to get changes within seconds and a guaranteed snapshot every hour.

## Attributes

| Name | Type | Inclusion | Description | Default |
| --- | --- | --- | --- | --- |
| `bartender_service_name` | string | Required | Name of the bartender generic service. | |
| `heartbeat_sec` | int | Optional | Max seconds between snapshots when nothing changes. | 3600 |

## Readings

Same shape as the bartender's `get_inventory` response body:

```json
{
  "ingredients": {
    "gin": { "in_stock": true, "slot": "slot-1", "pourer_oz": 1.5, "category": "Gin" }
  },
  "slots": [{ "id": "slot-1", "ingredient": "gin" }]
}
```

## Example configuration

Requires a `data_manager` service with sync enabled on the machine.

```json
{
  "name": "inventory",
  "api": "rdk:component:sensor",
  "model": "viam:cocktail-bot:inventory-sensor",
  "attributes": {
    "bartender_service_name": "bartender"
  },
  "service_configs": [
    {
      "type": "data_manager",
      "attributes": {
        "capture_methods": [
          { "method": "Readings", "capture_frequency_hz": 0.1 }
        ]
      }
    }
  ]
}
```
