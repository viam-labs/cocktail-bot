package bartender

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/rdk/logging"
	"go.viam.com/test"
)

func newCleanerPowerBartender(t *testing.T, name string, sw *recordingSwitch, slackURL string) *bartender {
	logger, _ := logging.NewObservedTestLogger(t)
	b := &bartender{
		logger: logger,
		cfg: &Config{
			SlackWebhookURL: slackURL,
			ShakerCleaning:  &ShakerCleaning{Enabled: true, PowerSwitchName: name},
		},
	}
	if sw != nil {
		b.cleanerPower = sw
	}
	return b
}

func TestStartCleanerUnconfiguredIsNoop(t *testing.T) {
	b := newCleanerPowerBartender(t, "", nil, "")
	stop, err := b.startCleaner(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stop(), test.ShouldBeNil)
}

func TestStartCleanerPowersOnThenOffOnce(t *testing.T) {
	sw := newRecordingSwitch(nil)
	b := newCleanerPowerBartender(t, "cleaner-power", sw, "")

	stop, err := b.startCleaner(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sw.calls(), test.ShouldResemble, []uint32{1})

	test.That(t, stop(), test.ShouldBeNil)
	test.That(t, stop(), test.ShouldBeNil)
	test.That(t, sw.calls(), test.ShouldResemble, []uint32{1, 0})
}

func TestStartCleanerReturnsErrorAndSlacksOnPowerOnFailure(t *testing.T) {
	slackURL, texts := slackRecorder(t)
	sw := newRecordingSwitch(errors.New("switch unreachable"))
	b := newCleanerPowerBartender(t, "cleaner-power", sw, slackURL)

	stop, err := b.startCleaner(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "power on cleaner")
	test.That(t, stop, test.ShouldNotBeNil)

	msg := nextSlack(t, texts)
	test.That(t, msg, test.ShouldContainSubstring, "Couldn't turn the cleaner on")
	test.That(t, msg, test.ShouldContainSubstring, "order has been failed")
	test.That(t, msg, test.ShouldContainSubstring, "switch unreachable")
}

func TestStopCleanerReturnsErrorAndSlacksOnPowerOffFailure(t *testing.T) {
	slackURL, texts := slackRecorder(t)
	calls := 0
	sw := newRecordingSwitch(nil)
	// first call (on) succeeds, second (off) fails
	sw.SetPositionFunc = func(ctx context.Context, position uint32, _ map[string]any) error {
		sw.mu.Lock()
		sw.positions = append(sw.positions, position)
		sw.mu.Unlock()
		calls++
		if calls == 2 {
			return errors.New("switch stuck on")
		}
		return nil
	}
	b := newCleanerPowerBartender(t, "cleaner-power", sw, slackURL)

	stop, err := b.startCleaner(context.Background())
	test.That(t, err, test.ShouldBeNil)
	stopErr := stop()
	test.That(t, stopErr, test.ShouldNotBeNil)
	test.That(t, stopErr.Error(), test.ShouldContainSubstring, "power off cleaner")

	msg := nextSlack(t, texts)
	test.That(t, msg, test.ShouldContainSubstring, "Couldn't turn the cleaner off")
	test.That(t, msg, test.ShouldContainSubstring, "switch stuck on")

	// Subsequent stop() calls are no-ops (sync.Once); they still return the cached error.
	test.That(t, stop(), test.ShouldEqual, stopErr)
}
