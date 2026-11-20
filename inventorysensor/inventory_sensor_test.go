package inventorysensor

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/test"
)

type fakeBartender struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	inventory map[string]any
	err       error
}

func (f *fakeBartender) DoCommand(_ context.Context, cmd map[string]any) (map[string]any, error) {
	if f.err != nil {
		return nil, f.err
	}
	if _, ok := cmd["get_inventory"]; ok {
		return map[string]any{"inventory": f.inventory}, nil
	}
	return nil, errors.New("unknown command")
}

type profile struct {
	ABV float64 `json:"abv"`
}

func setup(t *testing.T) (*inventorySensor, *fakeBartender, *time.Time) {
	t.Helper()
	clock := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	b := &fakeBartender{
		Named: resource.NewName(sensor.API, "bartender").AsNamed(),
		inventory: map[string]any{
			"ingredients": map[string]any{
				"gin": map[string]any{"in_stock": true, "slot": "slot-1", "whiskey_profile": profile{ABV: 40}},
			},
			"slots": []any{map[string]any{"id": "slot-1", "ingredient": "gin"}},
		},
	}
	s := &inventorySensor{
		name:      resource.NewName(sensor.API, "inventory"),
		logger:    logging.NewTestLogger(t),
		bartender: b,
		heartbeat: time.Hour,
		now:       func() time.Time { return clock },
	}
	return s, b, &clock
}

func TestReadings_FirstCallEmitsSnapshot(t *testing.T) {
	s, _, _ := setup(t)
	readings, err := s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldBeNil)
	gin := readings["ingredients"].(map[string]any)["gin"].(map[string]any)
	test.That(t, gin["slot"], test.ShouldEqual, "slot-1")
	test.That(t, gin["whiskey_profile"], test.ShouldResemble, map[string]any{"abv": 40.0})
	test.That(t, readings["slots"], test.ShouldHaveLength, 1)
}

func TestReadings_SkipsUnchangedSnapshot(t *testing.T) {
	s, _, clock := setup(t)
	_, err := s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldBeNil)
	*clock = clock.Add(time.Minute)
	_, err = s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldEqual, data.ErrNoCaptureToStore)
}

func TestReadings_EmitsOnChange(t *testing.T) {
	s, b, _ := setup(t)
	_, err := s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldBeNil)
	b.inventory["slots"] = []any{map[string]any{"id": "slot-1", "ingredient": "rum"}}
	readings, err := s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldBeNil)
	slot := readings["slots"].([]any)[0].(map[string]any)
	test.That(t, slot["ingredient"], test.ShouldEqual, "rum")
}

func TestReadings_EmitsOnHeartbeat(t *testing.T) {
	s, _, clock := setup(t)
	_, err := s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldBeNil)
	*clock = clock.Add(time.Hour)
	_, err = s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldBeNil)
}

func TestReadings_BartenderErrorPassesThrough(t *testing.T) {
	s, b, _ := setup(t)
	b.err = errors.New("data_dir not configured")
	_, err := s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "data_dir not configured")
}

func TestReadings_ErrorDoesNotResetDedup(t *testing.T) {
	s, b, _ := setup(t)
	_, err := s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldBeNil)
	b.err = errors.New("boom")
	_, err = s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldNotBeNil)
	b.err = nil
	_, err = s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldEqual, data.ErrNoCaptureToStore)
}

func TestValidate(t *testing.T) {
	_, _, err := (&Config{}).Validate("path")
	test.That(t, err, test.ShouldNotBeNil)

	_, _, err = (&Config{BartenderServiceName: "bartender", HeartbeatSec: -1}).Validate("path")
	test.That(t, err, test.ShouldNotBeNil)

	deps, _, err := (&Config{BartenderServiceName: "bartender"}).Validate("path")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldResemble, []string{"rdk:service:generic/bartender"})
}

func TestHeartbeatDefault(t *testing.T) {
	test.That(t, (&Config{}).heartbeat(), test.ShouldEqual, time.Hour)
	test.That(t, (&Config{HeartbeatSec: 60}).heartbeat(), test.ShouldEqual, time.Minute)
}

func TestReadings_NonDMCallerAlwaysGetsSnapshotWithoutConsumingChange(t *testing.T) {
	s, b, _ := setup(t)
	_, err := s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldBeNil)

	readings, err := s.Readings(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["slots"], test.ShouldHaveLength, 1)

	b.inventory["slots"] = []any{map[string]any{"id": "slot-1", "ingredient": "rum"}}
	_, err = s.Readings(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)

	_, err = s.Readings(context.Background(), data.FromDMExtraMap)
	test.That(t, err, test.ShouldBeNil)
}
