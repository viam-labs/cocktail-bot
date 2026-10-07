package bartender

import (
	"context"
	"fmt"
	"time"

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
	if raw, ok := cmd["pickup_pour_return"]; ok {
		return b.handlePickupPourReturn(ctx, raw)
	}
	if raw, ok := cmd["dispense_ice"]; ok {
		return b.handleDispenseIce(ctx, raw)
	}
	if raw, ok := cmd["pour_into_glasses"]; ok {
		return b.handlePourIntoGlasses(ctx, raw)
	}
	if _, ok := cmd["find_glass"]; ok {
		return b.handleFindGlass(ctx)
	}
	if raw, ok := cmd["find_and_pour"]; ok {
		return b.handleFindAndPour(ctx, raw)
	}
	return nil, fmt.Errorf("unknown command, supported: prepare_order, get_queue, execute_action, pickup_pour_return, dispense_ice, pour_into_glasses, find_glass, find_and_pour")
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
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	duration, err := b.moveArmToPose(ctx, pose)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"pose":        pose,
		"duration_ms": duration.Milliseconds(),
	}, nil
}

func (b *bartender) handlePickupPourReturn(ctx context.Context, raw any) (map[string]any, error) {
	bottle, pourMs, err := parsePickupPourReturn(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	start := time.Now()
	if err := b.pickupPourReturn(ctx, bottle, pourMs); err != nil {
		return nil, err
	}
	return map[string]any{
		"bottle":      bottle,
		"pour_ms":     pourMs,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func (b *bartender) handlePourIntoGlasses(ctx context.Context, raw any) (map[string]any, error) {
	req, err := parsePourIntoGlasses(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	start := time.Now()
	if err := b.pourIntoGlasses(ctx, req); err != nil {
		return nil, err
	}
	return map[string]any{
		"bottle":      req.bottle,
		"pour_ms":     req.pourMs,
		"glasses":     len(req.glasses),
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func foundGlassResponse(g foundGlass) map[string]any {
	return map[string]any{
		"x":        g.center.X,
		"y":        g.center.Y,
		"z":        g.center.Z,
		"label":    g.label,
		"lower_mm": g.lowerMM,
	}
}

func (b *bartender) handleFindGlass(ctx context.Context) (map[string]any, error) {
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	glass, err := b.findGlass(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"glass": foundGlassResponse(glass)}, nil
}

func (b *bartender) handleFindAndPour(ctx context.Context, raw any) (map[string]any, error) {
	bottle, pourMs, err := parsePickupPourReturn(raw)
	if err != nil {
		return nil, fmt.Errorf("find_and_pour: %w", err)
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	start := time.Now()
	glass, err := b.findAndPour(ctx, bottle, pourMs)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"glass":       foundGlassResponse(glass),
		"bottle":      bottle,
		"pour_ms":     pourMs,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func parsePickupPourReturn(raw any) (string, int, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", 0, fmt.Errorf("pickup_pour_return: expected object with 'bottle' and 'pour_ms', got %T", raw)
	}
	bottle, _ := m["bottle"].(string)
	if bottle == "" {
		return "", 0, fmt.Errorf("pickup_pour_return: 'bottle' is required")
	}
	var pourMs int
	switch v := m["pour_ms"].(type) {
	case float64:
		pourMs = int(v)
	case int:
		pourMs = v
	default:
		return "", 0, fmt.Errorf("pickup_pour_return: 'pour_ms' must be a number")
	}
	if pourMs < 0 {
		return "", 0, fmt.Errorf("pickup_pour_return: 'pour_ms' must be >= 0")
	}
	return bottle, pourMs, nil
}

func (b *bartender) handleDispenseIce(ctx context.Context, raw any) (map[string]any, error) {
	station, dwellMs, err := parseDispenseIce(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	start := time.Now()
	if err := b.dispenseIce(ctx, station, dwellMs); err != nil {
		return nil, err
	}
	return map[string]any{
		"station":     station,
		"dwell_ms":    dwellMs,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func parseDispenseIce(raw any) (string, int, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", 0, fmt.Errorf("dispense_ice: expected object with 'station' and 'dwell_ms', got %T", raw)
	}
	station, _ := m["station"].(string)
	if station == "" {
		return "", 0, fmt.Errorf("dispense_ice: 'station' is required")
	}
	var dwellMs int
	switch v := m["dwell_ms"].(type) {
	case float64:
		dwellMs = int(v)
	case int:
		dwellMs = v
	default:
		return "", 0, fmt.Errorf("dispense_ice: 'dwell_ms' must be a number")
	}
	if dwellMs < 0 {
		return "", 0, fmt.Errorf("dispense_ice: 'dwell_ms' must be >= 0")
	}
	return station, dwellMs, nil
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
