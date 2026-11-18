package bartender

import (
	"context"
	"fmt"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

type cocktailJob struct {
	plan     drinkPlan
	quantity int
}

func (b *bartender) prepareDrink(ctx context.Context, o order.Order) error {
	job, ok := o.Payload.(cocktailJob)
	if !ok {
		return fmt.Errorf("order %s has no recipe to make", o.ID)
	}
	b.logger.Infow("starting order", "order_id", o.ID, "drink", o.Drink)
	for i := 0; i < job.quantity; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := b.brew(ctx, job.plan); err != nil {
			return err
		}
	}
	b.logger.Infow("finished order", "order_id", o.ID)
	return nil
}
