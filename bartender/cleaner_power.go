package bartender

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const cleanerPowerTimeout = 10 * time.Second

// startCleaner powers the cleaner on and returns an idempotent stop closure.
// Failure to power on is returned immediately so the caller can fail the order;
// failure to power off (returned via the stop closure, or via the deferred safety
// call) likewise bubbles. Every failure also fires a Slack alert.
func (b *bartender) startCleaner(ctx context.Context) (func() error, error) {
	if b.cleanerPower == nil {
		return func() error { return nil }, nil
	}
	if err := b.setCleanerPower(ctx, true); err != nil {
		b.notifyCleanerPowerFail(ctx, true, err)
		return func() error { return nil }, fmt.Errorf("power on cleaner: %w", err)
	}
	var (
		once    sync.Once
		stopErr error
	)
	return func() error {
		once.Do(func() {
			if err := b.setCleanerPower(ctx, false); err != nil {
				b.notifyCleanerPowerFail(ctx, false, err)
				stopErr = fmt.Errorf("power off cleaner: %w", err)
			}
		})
		return stopErr
	}, nil
}

func (b *bartender) setCleanerPower(ctx context.Context, on bool) error {
	if b.cleanerPower == nil || (on && ctx.Err() != nil) {
		return nil
	}
	if !on {
		ctx = context.WithoutCancel(ctx)
	}
	ctx, cancel := context.WithTimeout(ctx, cleanerPowerTimeout)
	defer cancel()
	var pos uint32
	if on {
		pos = 1
	}
	return b.cleanerPower.SetPosition(ctx, pos, nil)
}

func (b *bartender) notifyCleanerPowerFail(ctx context.Context, on bool, err error) {
	action := "on"
	if !on {
		action = "off"
	}
	b.logger.Errorf("cleaner power %q: couldn't turn %s: %v", b.cleanerPowerSwitchName(), action, err)
	go slackPost(context.Background(), b.logger, b.slackWebhookURL(), cleanerPowerFailure(on, orderIDFromCtx(ctx), err), nil)
}

func (b *bartender) cleanerPowerSwitchName() string {
	if b.cfg.ShakerCleaning == nil {
		return ""
	}
	return b.cfg.ShakerCleaning.PowerSwitchName
}

func cleanerPowerFailure(on bool, orderID string, err error) string {
	action := "on"
	if !on {
		action = "off"
	}
	suffix := ""
	if orderID != "" {
		suffix = fmt.Sprintf(" [order %s]", escapeSlackMrkdwn(orderID))
	}
	return fmt.Sprintf(":x: Couldn't turn the cleaner %s — the order has been failed%s: %s", action, suffix, escapeSlackMrkdwn(err.Error()))
}
