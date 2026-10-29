package bartender

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const mixerPowerTimeout = 10 * time.Second

// startMixer powers the mixer on and returns an idempotent func that powers it off. Failures
// only warn, so an operator can switch the mixer by hand and the order still completes.
func (b *bartender) startMixer(ctx context.Context) func() {
	if b.mixerPower == nil {
		return func() {}
	}
	b.setMixerPowerOrWarn(ctx, true)
	var once sync.Once
	return func() { once.Do(func() { b.setMixerPowerOrWarn(ctx, false) }) }
}

func (b *bartender) setMixerPowerOrWarn(ctx context.Context, on bool) {
	err := b.setMixerPower(ctx, on)
	if err == nil {
		return
	}
	b.logger.Warnf("mixer power %q: %v", b.cfg.MixerPowerSwitchName, err)
	go slackPost(context.Background(), b.logger, b.slackWebhookURL(), mixerPowerWarning(on, orderIDFromCtx(ctx), err), nil)
}

func (b *bartender) setMixerPower(ctx context.Context, on bool) error {
	if b.mixerPower == nil || (on && ctx.Err() != nil) {
		return nil
	}
	// Powering off must still happen after an operator cancel.
	if !on {
		ctx = context.WithoutCancel(ctx)
	}
	ctx, cancel := context.WithTimeout(ctx, mixerPowerTimeout)
	defer cancel()
	var pos uint32
	if on {
		pos = 1
	}
	return b.mixerPower.SetPosition(ctx, pos, nil)
}

func mixerPowerWarning(on bool, orderID string, err error) string {
	action := "on — flip its switch on by hand"
	if !on {
		action = "off — switch it off by hand"
	}
	suffix := ""
	if orderID != "" {
		suffix = fmt.Sprintf(" [order %s]", escapeSlackMrkdwn(orderID))
	}
	return fmt.Sprintf(":warning: Couldn't turn the mixer %s%s: %s", action, suffix, escapeSlackMrkdwn(err.Error()))
}
