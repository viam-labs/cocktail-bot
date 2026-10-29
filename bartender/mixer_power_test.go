package bartender

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/viam-labs/cocktail-bot/bartender/filesaver"
	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

type recordingSwitch struct {
	*inject.Switch
	mu        sync.Mutex
	positions []uint32
	ctxErrs   []error
}

func newRecordingSwitch(err error) *recordingSwitch {
	r := &recordingSwitch{Switch: inject.NewSwitch("mixer-power")}
	r.SetPositionFunc = func(ctx context.Context, position uint32, _ map[string]any) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.positions = append(r.positions, position)
		r.ctxErrs = append(r.ctxErrs, ctx.Err())
		return err
	}
	return r
}

func (r *recordingSwitch) calls() []uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]uint32(nil), r.positions...)
}

func newMixerPowerBartender(t *testing.T, name string, sw *recordingSwitch, slackURL string) (*bartender, *observer.ObservedLogs) {
	logger, logs := logging.NewObservedTestLogger(t)
	b := &bartender{
		logger: logger,
		cfg:    &Config{MixerPowerSwitchName: name, SlackWebhookURL: slackURL},
	}
	if sw != nil {
		b.mixerPower = sw
	}
	return b, logs
}

func slackRecorder(t *testing.T) (string, <-chan string) {
	t.Helper()
	texts := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		texts <- body.Text
	}))
	t.Cleanup(srv.Close)
	return srv.URL, texts
}

func nextSlack(t *testing.T, texts <-chan string) string {
	t.Helper()
	select {
	case s := <-texts:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for slack post")
		return ""
	}
}

func TestStartMixerUnconfiguredIsNoop(t *testing.T) {
	b, _ := newMixerPowerBartender(t, "", nil, "")
	b.startMixer(context.Background())()
}

func TestStartMixerPowersOnThenOffOnce(t *testing.T) {
	sw := newRecordingSwitch(nil)
	b, logs := newMixerPowerBartender(t, "mixer-power", sw, "")

	stop := b.startMixer(context.Background())
	test.That(t, sw.calls(), test.ShouldResemble, []uint32{1})
	stop()
	stop()

	test.That(t, sw.calls(), test.ShouldResemble, []uint32{1, 0})
	test.That(t, logs.FilterMessageSnippet("mixer power").Len(), test.ShouldEqual, 0)
}

func TestStopMixerStillPowersOffAfterCancel(t *testing.T) {
	sw := newRecordingSwitch(nil)
	b, _ := newMixerPowerBartender(t, "mixer-power", sw, "")
	ctx, cancel := context.WithCancel(context.Background())

	stop := b.startMixer(ctx)
	cancel()
	stop()

	test.That(t, sw.calls(), test.ShouldResemble, []uint32{1, 0})
	test.That(t, sw.ctxErrs[1], test.ShouldBeNil)
}

func TestStartMixerSkipsPowerOnWhenAlreadyCancelled(t *testing.T) {
	sw := newRecordingSwitch(nil)
	b, logs := newMixerPowerBartender(t, "mixer-power", sw, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	b.startMixer(ctx)()

	test.That(t, sw.calls(), test.ShouldResemble, []uint32{0})
	test.That(t, logs.FilterMessageSnippet("mixer power").Len(), test.ShouldEqual, 0)
}

func TestMixerPowerFailureWarnsAndContinues(t *testing.T) {
	url, texts := slackRecorder(t)
	sw := newRecordingSwitch(errors.New("did not report state ON"))
	b, logs := newMixerPowerBartender(t, "mixer-power", sw, url)
	ctx := ctxWithOrderID(context.Background(), "ord-123")

	stop := b.startMixer(ctx)
	on := nextSlack(t, texts)
	stop()
	off := nextSlack(t, texts)

	test.That(t, sw.calls(), test.ShouldResemble, []uint32{1, 0})
	test.That(t, on, test.ShouldContainSubstring, "Couldn't turn the mixer on")
	test.That(t, on, test.ShouldContainSubstring, "[order ord-123]")
	test.That(t, on, test.ShouldContainSubstring, "did not report state ON")
	test.That(t, off, test.ShouldContainSubstring, "Couldn't turn the mixer off")
	test.That(t, logs.FilterMessageSnippet(`mixer power "mixer-power"`).Len(), test.ShouldEqual, 2)
}

func TestMixerPowerWarningEscapesSlackMarkup(t *testing.T) {
	got := mixerPowerWarning(true, "<!channel>", errors.New("<b>"))
	test.That(t, got, test.ShouldNotContainSubstring, "<")
}

func TestCloseCancelsRunningOrderThenPowersOffMixer(t *testing.T) {
	sw := newRecordingSwitch(nil)
	b, _ := newMixerPowerBartender(t, "mixer-power", sw, "")
	b.queueStop = make(chan struct{})
	b.filesaver = filesaver.New("", b.logger)
	orderCtx, done := b.withCancel(context.Background())
	defer done()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	test.That(t, b.Close(ctx), test.ShouldBeNil)

	test.That(t, orderCtx.Err(), test.ShouldNotBeNil)
	test.That(t, sw.calls(), test.ShouldResemble, []uint32{0})
	test.That(t, sw.ctxErrs[0], test.ShouldBeNil)
}

func TestConfigValidateMixerPowerSwitchIsRequiredDep(t *testing.T) {
	cfg := &Config{ArmName: "arm", GripperName: "gripper", PoseSwitcherNames: []string{"mixer"}, MixerPowerSwitchName: "mixer-power"}
	deps, optional, err := cfg.Validate("svc")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldContain, "rdk:component:switch/mixer-power")
	test.That(t, optional, test.ShouldNotContain, "rdk:component:switch/mixer-power")
}
