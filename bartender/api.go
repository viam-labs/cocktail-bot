package bartender

import (
	"context"
	"fmt"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

func (b *bartender) DoCommand(_ context.Context, cmd map[string]any) (map[string]any, error) {
	if raw, ok := cmd["prepare_order"]; ok {
		return b.handlePrepareOrder(raw)
	}
	if _, ok := cmd["get_queue"]; ok {
		return b.handleGetQueue()
	}
	return nil, fmt.Errorf("unknown command, supported: prepare_order, get_queue")
}

func (b *bartender) handlePrepareOrder(raw any) (map[string]any, error) {
	req, err := order.DecodeRequest(raw)
	if err != nil {
		return nil, err
	}
	o := order.NewOrder(req.Drink)
	b.queue.Enqueue(o)
	return map[string]any{
		"order_id": o.ID,
		"drink":    o.Drink,
	}, nil
}

func (b *bartender) handleGetQueue() (map[string]any, error) {
	st := b.queue.State()
	return map[string]any{
		"is_busy": st.IsBusy,
		"count":   st.Count,
		"current": st.Current,
		"queue":   st.Queue,
		"recent":  st.Recent,
	}, nil
}
