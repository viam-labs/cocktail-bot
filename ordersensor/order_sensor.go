package ordersensor

import (
	"context"
	"sync"
	"time"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

var Model = resource.NewModel("viam", "cocktail-bot", "order-sensor")

func init() {
	resource.RegisterComponent(sensor.API, Model,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: newOrderSensor,
		})
}

type Config struct{}

func (c *Config) Validate(string) ([]string, []string, error) {
	return nil, nil, nil
}

type orderSensor struct {
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	name   resource.Name
	logger logging.Logger

	mu      sync.Mutex
	pending []map[string]any
}

func newOrderSensor(_ context.Context, _ resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	return &orderSensor{
		name:   rawConf.ResourceName(),
		logger: logger,
	}, nil
}

func (s *orderSensor) Name() resource.Name { return s.name }

func (s *orderSensor) Status(context.Context) (map[string]any, error) {
	return map[string]any{}, nil
}

func (s *orderSensor) Readings(_ context.Context, _ map[string]any) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) == 0 {
		return nil, data.ErrNoCaptureToStore
	}
	payload := s.pending[0]
	s.pending[0] = nil
	s.pending = s.pending[1:]
	return payload, nil
}

func (*orderSensor) DoCommand(_ context.Context, _ map[string]any) (map[string]any, error) {
	return nil, nil
}

func (s *orderSensor) PushOrderReading(r order.Reading) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = append(s.pending, map[string]any{
		"order_id":      r.OrderID,
		"drink":         r.Drink,
		"status":        string(r.Status),
		"error_message": r.ErrMessage,
		"failed_step":   r.FailedStep,
		"start_time":    r.StartedAt.UTC().Format(time.RFC3339Nano),
		"end_time":      r.EndedAt.UTC().Format(time.RFC3339Nano),
	})
}
