package maintenancesensor

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

type fakeBartenderService struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	isBusy     bool
	queueCount float64
}

func (f *fakeBartenderService) DoCommand(_ context.Context, cmd map[string]any) (map[string]any, error) {
	if _, ok := cmd["get_queue"]; ok {
		return map[string]any{
			"is_busy": f.isBusy,
			"count":   f.queueCount,
		}, nil
	}
	return nil, errors.New("unknown command")
}

type testOpts struct {
	armMoving  bool
	isBusy     bool
	queueCount float64
}

func setupMaintenanceSensor(t *testing.T, opts testOpts) (sensor.Sensor, *inject.Arm, *fakeBartenderService) {
	t.Helper()

	logger := logging.NewTestLogger(t)

	fakeArm := &inject.Arm{}
	fakeArm.IsMovingFunc = func(ctx context.Context) (bool, error) {
		return opts.armMoving, nil
	}

	bartender := &fakeBartenderService{
		Named:      resource.NewName(sensor.API, "bartender").AsNamed(),
		isBusy:     opts.isBusy,
		queueCount: opts.queueCount,
	}

	s := &maintenanceSensor{
		name:      resource.NewName(sensor.API, "maintenance"),
		logger:    logger,
		bartender: bartender,
		arm:       fakeArm,
	}
	return s, fakeArm, bartender
}

func TestReadings_Safe_WhenIdle(t *testing.T) {
	s, _, _ := setupMaintenanceSensor(t, testOpts{})
	readings, err := s.Readings(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["is_safe"], test.ShouldBeTrue)
}

func TestReadings_Unsafe_WhenArmMoving(t *testing.T) {
	s, _, _ := setupMaintenanceSensor(t, testOpts{armMoving: true})
	readings, err := s.Readings(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["is_safe"], test.ShouldBeFalse)
}

func TestReadings_Unsafe_WhenSequenceRunning(t *testing.T) {
	s, _, _ := setupMaintenanceSensor(t, testOpts{isBusy: true})
	readings, err := s.Readings(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["is_safe"], test.ShouldBeFalse)
}

func TestReadings_Unsafe_WhenQueueHasOrders(t *testing.T) {
	s, _, _ := setupMaintenanceSensor(t, testOpts{queueCount: 2})
	readings, err := s.Readings(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["is_safe"], test.ShouldBeFalse)
}

func TestReadings_Unsafe_WhenAllActive(t *testing.T) {
	s, _, _ := setupMaintenanceSensor(t, testOpts{armMoving: true, isBusy: true, queueCount: 1})
	readings, err := s.Readings(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["is_safe"], test.ShouldBeFalse)
}

func TestReadings_Error_WhenArmFails(t *testing.T) {
	s, fakeArm, _ := setupMaintenanceSensor(t, testOpts{})
	fakeArm.IsMovingFunc = func(ctx context.Context) (bool, error) {
		return false, errors.New("arm unreachable")
	}
	_, err := s.Readings(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
}
