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
	for {
		select {
		case <-b.queueStop:
			return
		case <-b.queue.Notify():
		}
		for {
			o, ok := b.queue.Start()
			if !ok {
				break
			}
			b.safeExecuteOrder(o)
			b.queue.Complete()
		}
	}
}

func (b *bartender) safeExecuteOrder(o order.Order) {
	start := time.Now()
	var execErr error
	var failedStep string
	defer func() {
		if r := recover(); r != nil {
			execErr = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
		}
		b.publishReading(o, start, time.Now(), execErr, failedStep)
	}()
	ctx := ctxWithOrderID(context.Background(), o.ID)
	failedStep, execErr = b.prepareDrink(ctx, o)
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
