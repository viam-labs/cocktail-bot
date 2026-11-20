package bartender

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
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
	// The hand strainer while it rides on the held shaker, between strainer-seat on the way out and back.
	heldStrainerFrame referenceframe.Frame
	parkedGeomFrame   referenceframe.Frame
	// Glass obstacle at the serving station; the other glass obstacles are left out of planning.
	stationObstacle string
	filesaver       *filesaver.Saver
	queue           *order.Queue
	orderSink       orderSensorSink
	glassFinder     vision.Service
	mixerPower      toggleswitch.Switch
	cleanerPower    toggleswitch.Switch
	queueStop       chan struct{}
	queueDone       chan struct{}
	dataStore       *dataStore

	// runMu makes queued drinks and manual commands mutually exclusive on the arm.
	runMu      sync.Mutex
	cancelMu   sync.Mutex
	cancelFunc context.CancelFunc

	status statusTracker

	brew func(context.Context, drinkPlan) error
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

var errBusy = errors.New("bartender is busy with a drink or another command; wait for it to finish or cancel it")

// claim gives a manual command the arm, refusing rather than waiting if a drink is being made.
func (b *bartender) claim(ctx context.Context) (context.Context, func(), error) {
	if !b.runMu.TryLock() {
		return nil, nil, errBusy
	}
	ctx, done := b.withCancel(ctx)
	return ctx, func() {
		done()
		b.runMu.Unlock()
	}, nil
}

func (b *bartender) setPhase(phase string) {
	b.status.setPhase(phase)
	b.queue.SetStep(phase)
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
		queueDone:    make(chan struct{}),

		stationObstacle: cfg.DefaultGlassObstacle,
	}
	b.brew = b.makeCocktail

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
			logger.Warnf("optional glass finder %q not available: %v; bartender will come up without find_glass; a later reconfigure once the dep is ready will attach it",
				cfg.GlassFinderName, err)
		} else {
			b.glassFinder = gf
		}
	}

	if cfg.MixerPowerSwitchName != "" {
		sw, err := toggleswitch.FromProvider(deps, cfg.MixerPowerSwitchName)
		if err != nil {
			return nil, fmt.Errorf("mixer power switch %q: %w", cfg.MixerPowerSwitchName, err)
		}
		b.mixerPower = sw
	}

	if sc := cfg.ShakerCleaning; sc != nil && sc.PowerSwitchName != "" {
		sw, err := toggleswitch.FromProvider(deps, sc.PowerSwitchName)
		if err != nil {
			return nil, fmt.Errorf("cleaner power switch %q: %w", sc.PowerSwitchName, err)
		}
		b.cleanerPower = sw
	}

	if dir := cfg.dataDir(); dir != "" {
		ds, err := newDataStore(dir)
		if err != nil {
			return nil, fmt.Errorf("data store: %w", err)
		}
		b.dataStore = ds
		q, interrupted, err := order.OpenQueue(order.NewFileStore(filepath.Join(dir, queueFileName)), func(err error) {
			logger.Errorw("save order queue; a restart may lose its latest change", "err", err)
		})
		if err != nil {
			return nil, fmt.Errorf("order queue: %w", err)
		}
		b.queue = q
		if interrupted != nil {
			b.reportInterrupted(*interrupted)
		}
	} else {
		logger.Warn("data_dir not set and VIAM_MODULE_DATA unset; recipes and inventory DoCommands unavailable, and the order queue is kept in memory only")
	}

	go b.processQueue()
	return b, nil
}

func (b *bartender) Close(ctx context.Context) error {
	close(b.queueStop)
	b.cancelRunning()
	select {
	case <-b.queueDone:
	case <-ctx.Done():
		b.logger.Warnf("close: order queue worker still running: %v", ctx.Err())
	}
	if err := b.setMixerPower(ctx, false); err != nil {
		b.logger.Warnf("power off mixer %q on close: %v", b.cfg.MixerPowerSwitchName, err)
	}
	if err := b.setCleanerPower(ctx, false); err != nil {
		b.logger.Warnf("power off cleaner %q on close: %v", b.cleanerPowerSwitchName(), err)
	}
	return b.filesaver.Close()
}
