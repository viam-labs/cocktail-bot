package bartender

import (
	"context"
	"fmt"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

func (b *bartender) DoCommand(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if raw, ok := cmd["prepare_order"]; ok {
		return b.handlePrepareOrder(raw)
	}
	if _, ok := cmd["get_queue"]; ok {
		return b.handleGetQueue()
	}
	if raw, ok := cmd["execute_action"]; ok {
		return b.handleExecuteAction(ctx, raw)
	}
	return nil, fmt.Errorf("unknown command, supported: prepare_order, get_queue, execute_action")
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

func (b *bartender) handleExecuteAction(ctx context.Context, raw any) (map[string]any, error) {
	pose, err := parseExecuteAction(raw)
	if err != nil {
		return nil, err
	}
	duration, err := b.moveArmToPose(ctx, pose)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"pose":        pose,
		"duration_ms": duration.Milliseconds(),
	}, nil
}

func parseExecuteAction(raw any) (string, error) {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return "", fmt.Errorf("execute_action: pose name is required")
		}
		return v, nil
	case map[string]any:
		pose, _ := v["pose"].(string)
		if pose == "" {
			return "", fmt.Errorf("execute_action: 'pose' is required")
		}
		return pose, nil
	default:
		return "", fmt.Errorf("execute_action: expected string or object with 'pose', got %T", raw)
	}
}
