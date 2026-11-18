package bartender

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

// cocktailJob is the order payload; it is saved with the queue, so its fields must round-trip through JSON.
type cocktailJob struct {
	Title      string       `json:"title"`
	Steps      []RecipeStep `json:"steps"`
	ServePhase string       `json:"serve_phase"`
	Double     bool         `json:"double"`
	Quantity   int          `json:"quantity"`
}

func newCocktailJob(plan drinkPlan, quantity int) (json.RawMessage, error) {
	return json.Marshal(cocktailJob{
		Title:      plan.title,
		Steps:      plan.steps,
		ServePhase: plan.servePhase,
		Double:     plan.double,
		Quantity:   quantity,
	})
}

func (b *bartender) prepareDrink(ctx context.Context, o order.Order) error {
	var job cocktailJob
	if len(o.Payload) == 0 {
		return fmt.Errorf("order %s has no recipe to make", o.ID)
	}
	if err := json.Unmarshal(o.Payload, &job); err != nil {
		return fmt.Errorf("order %s: decode recipe: %w", o.ID, err)
	}
	plan := drinkPlan{title: job.Title, steps: job.Steps, servePhase: job.ServePhase, double: job.Double}
	b.logger.Infow("starting order", "order_id", o.ID, "drink", o.Drink)
	for i := 0; i < job.Quantity; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := b.brew(ctx, plan); err != nil {
			return err
		}
	}
	b.logger.Infow("finished order", "order_id", o.ID)
	return nil
}
