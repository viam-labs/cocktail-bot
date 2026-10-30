// Package inventorysensor registers a viam:cocktail-bot:inventory-sensor model
// that implements the rdk:component:sensor API. Each reading is a snapshot of
// the bartender's inventory (stock and slot assignments) for Data Management
// to capture and sync.
package inventorysensor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	generic "go.viam.com/rdk/services/generic"
)

var Model = resource.NewModel("viam", "cocktail-bot", "inventory-sensor")

const defaultHeartbeat = time.Hour

func init() {
	resource.RegisterComponent(sensor.API, Model,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: newInventorySensor,
		},
	)
}

type Config struct {
	BartenderServiceName string `json:"bartender_service_name"`
	HeartbeatSec         int    `json:"heartbeat_sec,omitempty"`
}

func (cfg *Config) Validate(path string) ([]string, []string, error) {
	if cfg.BartenderServiceName == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "bartender_service_name")
	}
	if cfg.HeartbeatSec < 0 {
		return nil, nil, fmt.Errorf("%s: heartbeat_sec must be >= 0, got %d", path, cfg.HeartbeatSec)
	}
	return []string{resource.NewName(generic.API, cfg.BartenderServiceName).String()}, nil, nil
}

func (cfg *Config) heartbeat() time.Duration {
	if cfg.HeartbeatSec == 0 {
		return defaultHeartbeat
	}
	return time.Duration(cfg.HeartbeatSec) * time.Second
}

type inventorySensor struct {
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	name      resource.Name
	logger    logging.Logger
	bartender resource.Resource
	heartbeat time.Duration
	now       func() time.Time

	mu       sync.Mutex
	lastHash [sha256.Size]byte
	lastEmit time.Time
}

func newInventorySensor(_ context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}
	bartenderRes, ok := deps[resource.NewName(generic.API, conf.BartenderServiceName)]
	if !ok {
		return nil, fmt.Errorf("bartender service %q not found in dependencies", conf.BartenderServiceName)
	}
	return &inventorySensor{
		name:      rawConf.ResourceName(),
		logger:    logger,
		bartender: bartenderRes,
		heartbeat: conf.heartbeat(),
		now:       time.Now,
	}, nil
}

func (s *inventorySensor) Name() resource.Name { return s.name }

func (s *inventorySensor) Status(context.Context) (map[string]any, error) {
	return map[string]any{}, nil
}

// For Data Management calls, returns data.ErrNoCaptureToStore unless the
// snapshot changed or the heartbeat elapsed, so a high capture frequency
// doesn't fill the table with duplicates. Other callers always get the
// snapshot and don't affect dedup state.
func (s *inventorySensor) Readings(ctx context.Context, extra map[string]any) (map[string]any, error) {
	resp, err := s.bartender.DoCommand(ctx, map[string]any{"get_inventory": true})
	if err != nil {
		return nil, fmt.Errorf("get_inventory: %w", err)
	}
	inv, ok := resp["inventory"]
	if !ok {
		return nil, fmt.Errorf("get_inventory: response missing 'inventory'")
	}
	// JSON round-trip turns nested Go structs into proto-compatible maps and
	// gives a key-sorted encoding to hash.
	encoded, err := json.Marshal(inv)
	if err != nil {
		return nil, fmt.Errorf("marshal inventory: %w", err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return nil, fmt.Errorf("unmarshal inventory: %w", err)
	}
	if fromDM, _ := extra[data.FromDMString].(bool); !fromDM {
		return snapshot, nil
	}
	hash := sha256.Sum256(encoded)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if !s.lastEmit.IsZero() && hash == s.lastHash && now.Sub(s.lastEmit) < s.heartbeat {
		return nil, data.ErrNoCaptureToStore
	}
	s.lastHash = hash
	s.lastEmit = now
	return snapshot, nil
}

func (*inventorySensor) DoCommand(context.Context, map[string]any) (map[string]any, error) {
	return nil, nil
}
