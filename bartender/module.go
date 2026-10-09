package bartender

import (
	"context"
	"fmt"
	"sync"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/sensor"
	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/generic"
	"go.viam.com/rdk/services/vision"

	"github.com/viam-labs/cocktail-bot/bartender/filesaver"
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
	logger        logging.Logger
	cfg           *Config
	arm           arm.Arm
	gripper       gripper.Gripper
	fsSvc         framesystem.Service
	poseSwitches  []toggleswitch.Switch
	heldGeomFrame referenceframe.Frame
	filesaver     *filesaver.Saver
	queue         *order.Queue
	orderSink     orderSensorSink
	glassFinder   vision.Service
	queueStop     chan struct{}
	dataStore     *dataStore

	cancelMu   sync.Mutex
	cancelFunc context.CancelFunc

	status statusTracker
}

func (b *bartender) withCancel(ctx context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(ctx)
	b.cancelMu.Lock()
	if b.cancelFunc != nil {
		b.cancelFunc()
	}
	b.cancelFunc = cancel
	b.cancelMu.Unlock()
	return ctx, func() {
		b.cancelMu.Lock()
		if b.cancelFunc != nil {
			b.cancelFunc = nil
		}
		b.cancelMu.Unlock()
		cancel()
	}
}

func (b *bartender) cancelRunning() bool {
	b.cancelMu.Lock()
	defer b.cancelMu.Unlock()
	if b.cancelFunc == nil {
		return false
	}
	b.cancelFunc()
	b.cancelFunc = nil
	return true
}

func newBartender(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (resource.Resource, error) {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	armComp, err := arm.FromProvider(deps, cfg.ArmName)
	if err != nil {
		return nil, fmt.Errorf("arm %q: %w", cfg.ArmName, err)
	}

	gripperComp, err := gripper.FromProvider(deps, cfg.GripperName)
	if err != nil {
		return nil, fmt.Errorf("gripper %q: %w", cfg.GripperName, err)
	}

	fsSvc, err := framesystem.FromDependencies(deps)
	if err != nil {
		return nil, fmt.Errorf("frame system service: %w", err)
	}

	poseSwitches := make([]toggleswitch.Switch, 0, len(cfg.PoseSwitcherNames))
	for _, name := range cfg.PoseSwitcherNames {
		sw, err := toggleswitch.FromProvider(deps, name)
		if err != nil {
			return nil, fmt.Errorf("pose switcher %q: %w", name, err)
		}
		poseSwitches = append(poseSwitches, sw)
	}

	b := &bartender{
		Named:        conf.ResourceName().AsNamed(),
		logger:       logger,
		cfg:          cfg,
		arm:          armComp,
		gripper:      gripperComp,
		fsSvc:        fsSvc,
		poseSwitches: poseSwitches,
		filesaver:    filesaver.New(cfg.SaveMotionRequestsDir, logger),
		queue:        order.NewQueue(),
		queueStop:    make(chan struct{}),
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

	if cfg.GlassFinderName != "" {
		gf, err := vision.FromProvider(deps, cfg.GlassFinderName)
		if err != nil {
			return nil, fmt.Errorf("glass finder %q: %w", cfg.GlassFinderName, err)
		}
		b.glassFinder = gf
	}

	if dir := cfg.dataDir(); dir != "" {
		ds, err := newDataStore(dir)
		if err != nil {
			return nil, fmt.Errorf("data store: %w", err)
		}
		b.dataStore = ds
	} else {
		logger.Warn("data_dir not set and VIAM_MODULE_DATA unset; recipes and inventory DoCommands unavailable")
	}

	go b.processQueue()
	return b, nil
}

func (b *bartender) Close(_ context.Context) error {
	close(b.queueStop)
	return b.filesaver.Close()
}
