package bartender

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/generic"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

var Model = resource.NewModel("viam", "cocktail-bot", "bartender")

func init() {
	resource.RegisterService(generic.API, Model,
		resource.Registration[resource.Resource, *Config]{
			Constructor: newBartender,
		},
	)
}

type bartender struct {
	resource.Named
	resource.AlwaysRebuild
	logger    logging.Logger
	queue     *order.Queue
	orderSink orderSensorSink
	queueStop chan struct{}
}

func newBartender(_ context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (resource.Resource, error) {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	b := &bartender{
		Named:     conf.ResourceName().AsNamed(),
		logger:    logger,
		queue:     order.NewQueue(),
		queueStop: make(chan struct{}),
	}

	if cfg.OrderSensorName != "" {
		s, err := sensor.FromProvider(deps, cfg.OrderSensorName)
		if err != nil {
			return nil, fmt.Errorf("order sensor %q: %w", cfg.OrderSensorName, err)
		}
		sink, ok := s.(orderSensorSink)
		if !ok {
			return nil, fmt.Errorf("order sensor %q does not implement PushOrderReading", cfg.OrderSensorName)
		}
		b.orderSink = sink
	} else {
		logger.Warn("order_sensor_name not set; order history will not be persisted")
	}

	go b.processQueue()
	return b, nil
}

func (b *bartender) Close(_ context.Context) error {
	close(b.queueStop)
	return nil
}
