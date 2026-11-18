package bartender

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

type orderSensorSink interface {
	PushOrderReading(r order.Reading)
}

func (b *bartender) processQueue() {
	defer close(b.queueDone)
	life, stop := context.WithCancel(context.Background())
	defer stop()
	go func() {
		select {
		case <-b.queueStop:
			stop()
		case <-life.Done():
		}
	}()
	for {
		select {
		case <-life.Done():
			return
		case <-b.queue.Notify():
		}
		for b.runNext(life) {
		}
	}
}

// runNext takes the arm before popping an order, so neither get_queue nor cancel
// treats an order as running while a manual command still holds the arm.
func (b *bartender) runNext(life context.Context) bool {
	b.runMu.Lock()
	defer b.runMu.Unlock()
	if life.Err() != nil {
		return false
	}
	o, ok := b.queue.Start()
	if !ok {
		return false
	}
	ctx, done := b.withCancel(ctxWithOrderID(life, o.ID))
	defer done()
	b.queue.Complete(b.safeExecuteOrder(ctx, o))
	return true
}

func (b *bartender) safeExecuteOrder(ctx context.Context, o order.Order) (execErr error) {
	b.status.begin(o.Drink)
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			execErr = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
		}
		end := time.Now()
		failedStep := ""
		if snap := b.status.snapshot(); len(snap.history) > 0 {
			failedStep = snap.history[len(snap.history)-1].Step
		}
		b.status.finish(execErr)
		b.publishReading(o, start, end, execErr, failedStep)
		alert := orderAlert{
			drink:             o.Drink,
			double:            o.Double,
			orderID:           o.ID,
			phase:             failedStep,
			duration:          end.Sub(start),
			err:               execErr,
			operatorCancelled: isOperatorCancel(execErr),
			machineID:         b.slackMachineID(),
		}
		slackPost(context.Background(), b.logger, b.slackWebhookURL(), alertText(alert), alertBlocks(alert))
	}()
	return b.prepareDrink(ctx, o)
}

func (b *bartender) publishReading(o order.Order, start, end time.Time, execErr error, failedStep string) {
	if b.orderSink == nil {
		return
	}
	status := order.StatusSucceeded
	errMsg := ""
	if execErr != nil {
		status = order.StatusFailed
		errMsg = execErr.Error()
	} else {
		failedStep = ""
	}
	b.orderSink.PushOrderReading(order.Reading{
		OrderID:    o.ID,
		Drink:      o.Drink,
		Status:     status,
		ErrMessage: errMsg,
		FailedStep: failedStep,
		StartedAt:  start,
		EndedAt:    end,
	})
}
