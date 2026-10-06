package bartender

import (
	"context"
	"time"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

var phaseDuration = time.Second

func (b *bartender) prepareDrink(ctx context.Context, o order.Order) (string, error) {
	b.logger.Infow("starting order", "order_id", o.ID, "drink", o.Drink)
	b.queue.SetStep("running")
	select {
	case <-ctx.Done():
		return "running", ctx.Err()
	case <-time.After(phaseDuration):
	}
	b.logger.Infow("finished order", "order_id", o.ID)
	return "", nil
}
