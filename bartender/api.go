package bartender

import (
	"context"
	"encoding/json"
	"errors"
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
	if _, ok := cmd["cancel"]; ok {
		return b.handleCancel()
	}
	if _, ok := cmd["get_status"]; ok {
		return b.handleGetStatus()
	}
	if raw, ok := cmd["execute_action"]; ok {
		return b.handleExecuteAction(ctx, raw)
	}
	if raw, ok := cmd["pickup_pour_return"]; ok {
		return b.handlePickupPourReturn(ctx, raw)
	}
	if raw, ok := cmd["grab_bottle"]; ok {
		return b.handleGrabBottle(ctx, raw)
	}
	if raw, ok := cmd["grab_shaker"]; ok {
		return b.handleGrabShaker(ctx, raw)
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
	if raw, ok := cmd["mix"]; ok {
		return b.handleMix(ctx, raw)
	}
	if raw, ok := cmd["stir"]; ok {
		return b.handleStir(ctx, raw)
	}
	if raw, ok := cmd["find_and_pour_from_shaker"]; ok {
		return b.handleFindAndPourFromShaker(ctx, raw)
	}
	if raw, ok := cmd["pour_from_shaker"]; ok {
		return b.handlePourFromShaker(ctx, raw)
	}
	if raw, ok := cmd["pour_into_shaker"]; ok {
		return b.handlePourIntoShaker(ctx, raw)
	}
	if raw, ok := cmd["strain_shaker"]; ok {
		return b.handleStrainShaker(ctx, raw)
	}
	if raw, ok := cmd["rotate_shakers"]; ok {
		return b.handleRotateShakers(ctx, raw)
	}
	if _, ok := cmd["get_recipes"]; ok {
		return b.handleGetRecipes()
	}
	if _, ok := cmd["get_pour_menus"]; ok {
		return b.handleGetPourMenus()
	}
	if _, ok := cmd["get_inventory"]; ok {
		return b.handleGetInventory()
	}
	if _, ok := cmd["get_bottles"]; ok {
		return b.handleGetBottles()
	}
	if raw, ok := cmd["update_inventory_item"]; ok {
		return b.handleUpdateInventoryItem(raw)
	}
	if raw, ok := cmd["delete_inventory_item"]; ok {
		return b.handleDeleteInventoryItem(raw)
	}
	if raw, ok := cmd["update_recipes"]; ok {
		return b.handleUpdateRecipes(raw)
	}
	if raw, ok := cmd["update_pour_menus"]; ok {
		return b.handleUpdatePourMenus(raw)
	}
	if raw, ok := cmd["update_slot_assignment"]; ok {
		return b.handleUpdateSlotAssignment(raw)
	}
	if raw, ok := cmd["clear_slot_assignment"]; ok {
		return b.handleClearSlotAssignment(raw)
	}
	if raw, ok := cmd["update_pourer_oz"]; ok {
		return b.handleUpdatePourerOz(raw)
	}
	if raw, ok := cmd["update_category"]; ok {
		return b.handleUpdateCategory(raw)
	}
	if raw, ok := cmd["update_whiskey_profile"]; ok {
		return b.handleUpdateWhiskeyProfile(raw)
	}
	if raw, ok := cmd["clear_whiskey_profile"]; ok {
		return b.handleClearWhiskeyProfile(raw)
	}
	if raw, ok := cmd["make_cocktail"]; ok {
		return b.handleMakeCocktail(ctx, raw)
	}
	return nil, fmt.Errorf("unknown command, supported: prepare_order, get_queue, get_status, cancel, execute_action, pickup_pour_return, grab_bottle, grab_shaker, dispense_ice, pour_into_glasses, find_glass, find_and_pour, find_and_pour_from_shaker, mix, stir, pour_from_shaker, pour_into_shaker, strain_shaker, rotate_shakers, get_recipes, get_pour_menus, get_inventory, get_bottles, update_inventory_item, delete_inventory_item, update_recipes, update_pour_menus, update_slot_assignment, clear_slot_assignment, update_pourer_oz, update_category, update_whiskey_profile, clear_whiskey_profile, make_cocktail")
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

func (b *bartender) handleCancel() (map[string]any, error) {
	cancelled := b.cancelRunning()
	b.detachHeld()
	return map[string]any{"cancelled": cancelled}, nil
}

func (b *bartender) handleGetStatus() (map[string]any, error) {
	snap := b.status.snapshot()
	resp := map[string]any{
		"is_busy":      snap.busy,
		"count":        float64(b.queue.State().Count),
		"current_step": "",
	}
	if snap.busy {
		resp["drink"] = snap.drink
		resp["started_at"] = snap.started.UTC().Format(time.RFC3339)
		resp["elapsed_ms"] = time.Since(snap.started).Milliseconds()
		if len(snap.history) > 0 {
			resp["current_step"] = snap.history[len(snap.history)-1].Step
		}
	}
	history := make([]any, len(snap.history))
	for i, e := range snap.history {
		history[i] = map[string]any{
			"step":       e.Step,
			"started_at": e.StartedAt.UTC().Format(time.RFC3339),
		}
	}
	resp["step_history"] = history
	if snap.lastOutcome != nil {
		out := map[string]any{
			"drink":    snap.lastOutcome.Drink,
			"ended_at": snap.lastOutcome.EndedAt.UTC().Format(time.RFC3339),
		}
		if snap.lastOutcome.Err != nil {
			out["error"] = snap.lastOutcome.Err.Error()
		}
		resp["last_outcome"] = out
	}
	return resp, nil
}

func (b *bartender) handleExecuteAction(ctx context.Context, raw any) (map[string]any, error) {
	pose, err := parseExecuteAction(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
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
	ctx, done := b.withCancel(ctx)
	defer done()
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

func (b *bartender) handleGrabBottle(ctx context.Context, raw any) (map[string]any, error) {
	bottle, err := parseGrabBottle(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	bottleSw, err := b.findSwitch(bottle)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	if err := b.pickupBottle(ctx, bottleSw); err != nil {
		return nil, err
	}
	return map[string]any{
		"bottle":      bottle,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func parseGrabBottle(raw any) (string, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", fmt.Errorf("grab_bottle: expected object with 'bottle', got %T", raw)
	}
	bottle, _ := m["bottle"].(string)
	if bottle == "" {
		return "", fmt.Errorf("grab_bottle: 'bottle' is required")
	}
	return bottle, nil
}

func (b *bartender) handleGrabShaker(ctx context.Context, raw any) (map[string]any, error) {
	switchName, source, err := parseGrabShaker(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	sw, err := b.findSwitch(switchName)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	if err := b.pickupShaker(ctx, sw, source); err != nil {
		return nil, err
	}
	return map[string]any{
		"source":      source,
		"switch":      switchName,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func parseGrabShaker(raw any) (string, string, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", "", fmt.Errorf("grab_shaker: expected object with 'switch' and 'source', got %T", raw)
	}
	switchName, _ := m["switch"].(string)
	if switchName == "" {
		return "", "", fmt.Errorf("grab_shaker: 'switch' is required (name of the pose switcher holding the shaker poses)")
	}
	source, _ := m["source"].(string)
	switch source {
	case "shaker-home", "shaker-a", "shaker-b":
		return switchName, source, nil
	case "":
		return "", "", fmt.Errorf("grab_shaker: 'source' is required (shaker-home, shaker-a, or shaker-b)")
	default:
		return "", "", fmt.Errorf("grab_shaker: 'source' must be shaker-home, shaker-a, or shaker-b, got %q", source)
	}
}

func (b *bartender) handlePourIntoGlasses(ctx context.Context, raw any) (map[string]any, error) {
	req, err := parsePourIntoGlasses(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
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
		"pan_deg":  g.panDeg,
	}
}

func (b *bartender) handleFindGlass(ctx context.Context) (map[string]any, error) {
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	glasses, err := b.findGlasses(ctx)
	if err != nil {
		return nil, err
	}
	all := make([]any, len(glasses))
	for i, g := range glasses {
		all[i] = foundGlassResponse(g)
	}
	return map[string]any{"glass": foundGlassResponse(glasses[0]), "glasses": all}, nil
}

func (b *bartender) handleFindAndPour(ctx context.Context, raw any) (map[string]any, error) {
	bottle, pourMs, err := parsePickupPourReturn(raw)
	if err != nil {
		return nil, fmt.Errorf("find_and_pour: %w", err)
	}
	mouthOffsetMM, err := parseMouthOffset(raw.(map[string]any))
	if err != nil {
		return nil, fmt.Errorf("find_and_pour: %w", err)
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	start := time.Now()
	glass, err := b.findAndPour(ctx, bottle, pourMs, mouthOffsetMM)
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
	ctx, done := b.withCancel(ctx)
	defer done()
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

func (b *bartender) handleMix(ctx context.Context, raw any) (map[string]any, error) {
	station, dwellMs, err := parseMix(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	start := time.Now()
	if err := b.mix(ctx, station, dwellMs); err != nil {
		return nil, err
	}
	return map[string]any{
		"station":     station,
		"dwell_ms":    dwellMs,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func parseMix(raw any) (string, int, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", 0, fmt.Errorf("mix: expected object with 'station' and 'dwell_ms', got %T", raw)
	}
	station, _ := m["station"].(string)
	if station == "" {
		return "", 0, fmt.Errorf("mix: 'station' is required")
	}
	var dwellMs int
	switch v := m["dwell_ms"].(type) {
	case float64:
		dwellMs = int(v)
	case int:
		dwellMs = v
	default:
		return "", 0, fmt.Errorf("mix: 'dwell_ms' must be a number")
	}
	if dwellMs < 0 {
		return "", 0, fmt.Errorf("mix: 'dwell_ms' must be >= 0")
	}
	return station, dwellMs, nil
}

func (b *bartender) handleStir(ctx context.Context, raw any) (map[string]any, error) {
	station, dwellMs, err := parseStir(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	start := time.Now()
	if err := b.stir(ctx, station, dwellMs); err != nil {
		return nil, err
	}
	return map[string]any{
		"station":     station,
		"dwell_ms":    dwellMs,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func parseStir(raw any) (string, int, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", 0, fmt.Errorf("stir: expected object with 'station' and 'dwell_ms', got %T", raw)
	}
	station, _ := m["station"].(string)
	if station == "" {
		return "", 0, fmt.Errorf("stir: 'station' is required")
	}
	var dwellMs int
	switch v := m["dwell_ms"].(type) {
	case float64:
		dwellMs = int(v)
	case int:
		dwellMs = v
	default:
		return "", 0, fmt.Errorf("stir: 'dwell_ms' must be a number")
	}
	if dwellMs < 0 {
		return "", 0, fmt.Errorf("stir: 'dwell_ms' must be >= 0")
	}
	return station, dwellMs, nil
}

func (b *bartender) handlePourFromShaker(ctx context.Context, raw any) (map[string]any, error) {
	station, pourMs, err := parsePourFromShaker(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	start := time.Now()
	if err := b.pourFromShaker(ctx, station, pourMs); err != nil {
		return nil, err
	}
	return map[string]any{
		"station":     station,
		"pour_ms":     pourMs,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func (b *bartender) handleFindAndPourFromShaker(ctx context.Context, raw any) (map[string]any, error) {
	pourMs, err := parseFindAndPourFromShaker(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	start := time.Now()
	poured, skipped, err := b.findAndPourFromShaker(ctx, pourMs)
	if err != nil {
		return nil, err
	}
	pouredResp := make([]any, len(poured))
	for i, g := range poured {
		pouredResp[i] = foundGlassResponse(g)
	}
	skippedResp := make([]any, len(skipped))
	for i, s := range skipped {
		r := foundGlassResponse(s.glass)
		r["reason"] = s.reason
		skippedResp[i] = r
	}
	return map[string]any{
		"glasses":     pouredResp,
		"skipped":     skippedResp,
		"station":     b.cfg.servingStation(),
		"pour_ms":     pourMs,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func parseFindAndPourFromShaker(raw any) (int, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return 0, fmt.Errorf("find_and_pour_from_shaker: expected object with 'pour_ms', got %T", raw)
	}
	var pourMs int
	switch v := m["pour_ms"].(type) {
	case float64:
		pourMs = int(v)
	case int:
		pourMs = v
	default:
		return 0, fmt.Errorf("find_and_pour_from_shaker: 'pour_ms' must be a number")
	}
	if pourMs < 0 {
		return 0, fmt.Errorf("find_and_pour_from_shaker: 'pour_ms' must be >= 0")
	}
	return pourMs, nil
}

func parsePourFromShaker(raw any) (string, int, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", 0, fmt.Errorf("pour_from_shaker: expected object with 'station' and 'pour_ms', got %T", raw)
	}
	station, _ := m["station"].(string)
	if station == "" {
		return "", 0, fmt.Errorf("pour_from_shaker: 'station' is required")
	}
	var pourMs int
	switch v := m["pour_ms"].(type) {
	case float64:
		pourMs = int(v)
	case int:
		pourMs = v
	default:
		return "", 0, fmt.Errorf("pour_from_shaker: 'pour_ms' must be a number")
	}
	if pourMs < 0 {
		return "", 0, fmt.Errorf("pour_from_shaker: 'pour_ms' must be >= 0")
	}
	return station, pourMs, nil
}

func (b *bartender) handlePourIntoShaker(ctx context.Context, raw any) (map[string]any, error) {
	bottle, oz, err := parsePourIntoShaker(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	start := time.Now()
	if err := b.pourIntoShaker(ctx, bottle, oz); err != nil {
		return nil, err
	}
	return map[string]any{
		"bottle":      bottle,
		"oz":          oz,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func parsePourIntoShaker(raw any) (string, float64, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", 0, fmt.Errorf("pour_into_shaker: expected object with 'bottle' and 'oz', got %T", raw)
	}
	bottle, _ := m["bottle"].(string)
	if bottle == "" {
		return "", 0, fmt.Errorf("pour_into_shaker: 'bottle' is required")
	}
	var oz float64
	switch v := m["oz"].(type) {
	case float64:
		oz = v
	case int:
		oz = float64(v)
	default:
		return "", 0, fmt.Errorf("pour_into_shaker: 'oz' must be a number")
	}
	if oz <= 0 {
		return "", 0, fmt.Errorf("pour_into_shaker: 'oz' must be > 0")
	}
	return bottle, oz, nil
}

func (b *bartender) handleGetRecipes() (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	return map[string]any{"recipes": b.dataStore.Recipes()}, nil
}

func (b *bartender) handleGetBottles() (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	assignments := b.dataStore.SlotAssignments()
	ingredients := make([]any, 0, len(assignments))
	for _, ingredient := range assignments {
		ingredients = append(ingredients, ingredient)
	}
	return map[string]any{"bottles": ingredients}, nil
}

func (b *bartender) handleGetInventory() (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	raw := b.dataStore.GetInventory()
	assignments := b.dataStore.SlotAssignments()
	pourerOz := b.dataStore.PourerOz()
	categories := b.dataStore.Categories()
	whiskeyProfiles := b.dataStore.WhiskeyProfiles()
	ingredientToSlot := make(map[string]string, len(assignments))
	for slot, ingredient := range assignments {
		ingredientToSlot[ingredient] = slot
	}
	decorate := func(entry map[string]any, ingredient string) {
		if oz, ok := pourerOz[ingredient]; ok {
			entry["pourer_oz"] = oz
		}
		if cat, ok := categories[ingredient]; ok && cat != "" {
			entry["category"] = cat
		}
		if wp, ok := whiskeyProfiles[ingredient]; ok {
			entry["whiskey_profile"] = wp
		}
	}
	ingredients := make(map[string]any, len(raw.Ingredients)+len(ingredientToSlot))
	for ingredient, stock := range raw.Ingredients {
		slot, hasSlot := ingredientToSlot[ingredient]
		entry := map[string]any{"in_stock": hasSlot && stock.InStock}
		if hasSlot {
			entry["slot"] = slot
		}
		decorate(entry, ingredient)
		ingredients[ingredient] = entry
	}
	for ingredient, slot := range ingredientToSlot {
		if _, seen := ingredients[ingredient]; seen {
			continue
		}
		entry := map[string]any{"in_stock": false, "slot": slot}
		decorate(entry, ingredient)
		ingredients[ingredient] = entry
	}
	slots := make([]any, 0, len(b.cfg.SlotNames))
	for _, slot := range b.cfg.SlotNames {
		entry := map[string]any{"id": slot}
		if ingredient, ok := assignments[slot]; ok {
			entry["ingredient"] = ingredient
		}
		slots = append(slots, entry)
	}
	return map[string]any{"inventory": map[string]any{"ingredients": ingredients, "slots": slots}}, nil
}

func (b *bartender) handleUpdateSlotAssignment(raw any) (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("update_slot_assignment: expected object with 'slot' and 'ingredient', got %T", raw)
	}
	slot, _ := m["slot"].(string)
	ingredient, _ := m["ingredient"].(string)
	if slot == "" {
		return nil, errors.New("update_slot_assignment: 'slot' is required")
	}
	if ingredient == "" {
		return nil, errors.New("update_slot_assignment: 'ingredient' is required; use clear_slot_assignment to empty a slot")
	}
	if !slotKnown(b.cfg.SlotNames, slot) {
		return nil, fmt.Errorf("update_slot_assignment: slot %q is not in slot_names", slot)
	}
	if err := b.dataStore.UpdateSlotAssignment(slot, ingredient); err != nil {
		return nil, err
	}
	return map[string]any{"slot": slot, "ingredient": ingredient}, nil
}

func (b *bartender) handleUpdatePourerOz(raw any) (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("update_pourer_oz: expected object with 'ingredient' and 'oz', got %T", raw)
	}
	ingredient, _ := m["ingredient"].(string)
	oz, _ := m["oz"].(float64)
	if ingredient == "" {
		return nil, errors.New("update_pourer_oz: 'ingredient' is required")
	}
	if oz <= 0 {
		return nil, errors.New("update_pourer_oz: 'oz' must be > 0")
	}
	if err := b.dataStore.UpdatePourerOz(ingredient, oz); err != nil {
		return nil, err
	}
	return map[string]any{"ingredient": ingredient, "oz": oz}, nil
}

func (b *bartender) handleUpdateCategory(raw any) (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("update_category: expected object with 'ingredient' and 'category', got %T", raw)
	}
	ingredient, _ := m["ingredient"].(string)
	category, _ := m["category"].(string)
	if ingredient == "" {
		return nil, errors.New("update_category: 'ingredient' is required")
	}
	if err := b.dataStore.UpdateCategory(ingredient, category); err != nil {
		return nil, err
	}
	return map[string]any{"ingredient": ingredient, "category": category}, nil
}

func (b *bartender) handleUpdateWhiskeyProfile(raw any) (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("update_whiskey_profile: expected object with 'ingredient' and 'profile', got %T", raw)
	}
	ingredient, _ := m["ingredient"].(string)
	if ingredient == "" {
		return nil, errors.New("update_whiskey_profile: 'ingredient' is required")
	}
	profileRaw, ok := m["profile"].(map[string]any)
	if !ok {
		return nil, errors.New("update_whiskey_profile: 'profile' must be an object")
	}
	profile := WhiskeyProfile{}
	if v, ok := profileRaw["abv"].(float64); ok {
		profile.ABV = v
	}
	if v, ok := profileRaw["region"].(string); ok {
		profile.Region = v
	}
	if v, ok := profileRaw["story"].(string); ok {
		profile.Story = v
	}
	if v, ok := profileRaw["notes"].(string); ok {
		profile.Notes = v
	}
	if v, ok := profileRaw["us"].(string); ok {
		profile.US = v
	}
	if v, ok := profileRaw["style"].(string); ok {
		profile.Style = v
	}
	if arr, ok := profileRaw["profile"].([]any); ok {
		scores := make([]int, 0, len(arr))
		for _, x := range arr {
			n, _ := x.(float64)
			scores = append(scores, int(n))
		}
		profile.Profile = scores
	}
	if err := b.dataStore.UpdateWhiskeyProfile(ingredient, profile); err != nil {
		return nil, err
	}
	return map[string]any{"ingredient": ingredient, "profile": profile}, nil
}

func (b *bartender) handleClearWhiskeyProfile(raw any) (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("clear_whiskey_profile: expected object with 'ingredient', got %T", raw)
	}
	ingredient, _ := m["ingredient"].(string)
	if ingredient == "" {
		return nil, errors.New("clear_whiskey_profile: 'ingredient' is required")
	}
	if err := b.dataStore.ClearWhiskeyProfile(ingredient); err != nil {
		return nil, err
	}
	return map[string]any{"ingredient": ingredient, "cleared": true}, nil
}

func (b *bartender) handleClearSlotAssignment(raw any) (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("clear_slot_assignment: expected object with 'slot', got %T", raw)
	}
	slot, _ := m["slot"].(string)
	if slot == "" {
		return nil, errors.New("clear_slot_assignment: 'slot' is required")
	}
	if err := b.dataStore.ClearSlotAssignment(slot); err != nil {
		return nil, err
	}
	return map[string]any{"slot": slot}, nil
}

func slotKnown(slots []string, want string) bool {
	for _, s := range slots {
		if s == want {
			return true
		}
	}
	return false
}

func (b *bartender) handleUpdateInventoryItem(raw any) (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	ingredient, inStock, err := parseUpdateInventoryItem(raw)
	if err != nil {
		return nil, err
	}
	if err := b.dataStore.UpdateInventoryItem(ingredient, inStock); err != nil {
		return nil, err
	}
	return map[string]any{"ingredient": ingredient, "in_stock": inStock}, nil
}

func (b *bartender) handleDeleteInventoryItem(raw any) (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("delete_inventory_item: expected object with 'ingredient', got %T", raw)
	}
	ingredient, _ := m["ingredient"].(string)
	if ingredient == "" {
		return nil, fmt.Errorf("delete_inventory_item: 'ingredient' is required")
	}
	if err := b.dataStore.DeleteInventoryItem(ingredient); err != nil {
		return nil, err
	}
	return map[string]any{"ingredient": ingredient, "deleted": true}, nil
}

func parseUpdateInventoryItem(raw any) (string, bool, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", false, fmt.Errorf("update_inventory_item: expected object with 'ingredient' and 'in_stock', got %T", raw)
	}
	ingredient, _ := m["ingredient"].(string)
	if ingredient == "" {
		return "", false, fmt.Errorf("update_inventory_item: 'ingredient' is required")
	}
	inStock, ok := m["in_stock"].(bool)
	if !ok {
		return "", false, fmt.Errorf("update_inventory_item: 'in_stock' must be a boolean")
	}
	return ingredient, inStock, nil
}

func (b *bartender) handleGetPourMenus() (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	return map[string]any{"pour_menus": b.dataStore.PourMenus()}, nil
}

func (b *bartender) handleUpdatePourMenus(raw any) (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	menus, err := parseUpdatePourMenus(raw)
	if err != nil {
		return nil, err
	}
	if err := b.dataStore.UpdatePourMenus(menus); err != nil {
		return nil, err
	}
	return map[string]any{"count": len(menus)}, nil
}

func parseUpdatePourMenus(raw any) ([]PourMenu, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("update_pour_menus: expected object with 'pour_menus', got %T", raw)
	}
	menusRaw, ok := m["pour_menus"]
	if !ok {
		return nil, fmt.Errorf("update_pour_menus: 'pour_menus' is required")
	}
	bytes, err := json.Marshal(menusRaw)
	if err != nil {
		return nil, fmt.Errorf("update_pour_menus: marshal pour_menus: %w", err)
	}
	var menus []PourMenu
	if err := json.Unmarshal(bytes, &menus); err != nil {
		return nil, fmt.Errorf("update_pour_menus: invalid pour_menus shape: %w", err)
	}
	return menus, nil
}

func (b *bartender) handleUpdateRecipes(raw any) (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	recipes, err := parseUpdateRecipes(raw)
	if err != nil {
		return nil, err
	}
	if err := b.dataStore.UpdateRecipes(recipes); err != nil {
		return nil, err
	}
	return map[string]any{"count": len(recipes)}, nil
}

func parseUpdateRecipes(raw any) ([]Recipe, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("update_recipes: expected object with 'recipes', got %T", raw)
	}
	recipesRaw, ok := m["recipes"]
	if !ok {
		return nil, fmt.Errorf("update_recipes: 'recipes' is required")
	}
	bytes, err := json.Marshal(recipesRaw)
	if err != nil {
		return nil, fmt.Errorf("update_recipes: marshal recipes: %w", err)
	}
	var recipes []Recipe
	if err := json.Unmarshal(bytes, &recipes); err != nil {
		return nil, fmt.Errorf("update_recipes: invalid recipes shape: %w", err)
	}
	return recipes, nil
}

func (b *bartender) handleStrainShaker(ctx context.Context, raw any) (map[string]any, error) {
	req, err := parseStrainShaker(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	start := time.Now()
	if err := b.strainShaker(ctx, req); err != nil {
		return nil, err
	}
	return map[string]any{
		"source":         req.source,
		"strain_flow":    req.strainFlow,
		"drain_dwell_ms": req.drainDwellMs,
		"dump_dwell_ms":  req.dumpDwellMs,
		"duration_ms":    time.Since(start).Milliseconds(),
	}, nil
}

func parseStrainShaker(raw any) (strainShakerRequest, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return strainShakerRequest{}, fmt.Errorf("strain_shaker: expected object, got %T", raw)
	}
	source, _ := m["source"].(string)
	if source == "" {
		return strainShakerRequest{}, fmt.Errorf("strain_shaker: 'source' is required")
	}
	strainFlow, _ := m["strain_flow"].(string)
	if strainFlow == "" {
		return strainShakerRequest{}, fmt.Errorf("strain_shaker: 'strain_flow' is required")
	}
	drainDwellMs, err := intField(m, "drain_dwell_ms")
	if err != nil {
		return strainShakerRequest{}, fmt.Errorf("strain_shaker: %w", err)
	}
	dumpDwellMs, err := intField(m, "dump_dwell_ms")
	if err != nil {
		return strainShakerRequest{}, fmt.Errorf("strain_shaker: %w", err)
	}
	return strainShakerRequest{
		source:       source,
		strainFlow:   strainFlow,
		drainDwellMs: drainDwellMs,
		dumpDwellMs:  dumpDwellMs,
	}, nil
}

func intField(m map[string]any, name string) (int, error) {
	v, ok := m[name]
	if !ok {
		return 0, fmt.Errorf("'%s' is required", name)
	}
	switch n := v.(type) {
	case float64:
		if n < 0 {
			return 0, fmt.Errorf("'%s' must be >= 0", name)
		}
		return int(n), nil
	case int:
		if n < 0 {
			return 0, fmt.Errorf("'%s' must be >= 0", name)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("'%s' must be a number", name)
	}
}

func (b *bartender) handleRotateShakers(ctx context.Context, raw any) (map[string]any, error) {
	strainFlow, err := parseRotateShakers(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	start := time.Now()
	if err := b.rotateShakers(ctx, strainFlow); err != nil {
		return nil, err
	}
	return map[string]any{
		"strain_flow": strainFlow,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func parseRotateShakers(raw any) (string, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", fmt.Errorf("rotate_shakers: expected object, got %T", raw)
	}
	strainFlow, _ := m["strain_flow"].(string)
	if strainFlow == "" {
		return "", fmt.Errorf("rotate_shakers: 'strain_flow' is required")
	}
	return strainFlow, nil
}

func (b *bartender) handleMakeCocktail(_ context.Context, raw any) (map[string]any, error) {
	drinkID, recipe, quantity, err := parseMakeCocktail(raw)
	if err != nil {
		return nil, err
	}
	if recipe == nil {
		if drinkID == "" {
			return nil, errors.New("make_cocktail: either drink_id or recipe is required")
		}
		if b.dataStore == nil {
			return nil, errDataStoreNotConfigured
		}
		for _, r := range b.dataStore.Recipes() {
			if r.ID == drinkID {
				rc := r
				recipe = &rc
				break
			}
		}
		if recipe == nil {
			return nil, fmt.Errorf("make_cocktail: recipe %q not found", drinkID)
		}
	}
	if b.cfg.RecipeDefaults == nil {
		return nil, errors.New("make_cocktail: recipe_defaults not configured on bartender service")
	}

	name := recipe.Name
	if name == "" {
		name = drinkID
	}
	displayName := name
	if quantity > 1 {
		displayName = fmt.Sprintf("%s × %d", name, quantity)
	}
	b.status.begin(displayName)
	start := time.Now()
	orderID := "manual-" + start.UTC().Format("20060102_150405")
	runCtx := ctxWithOrderID(context.Background(), orderID)
	runCtx, done := b.withCancel(runCtx)

	go func() {
		defer done()
		var runErr error
		for i := 0; i < quantity; i++ {
			runErr = b.makeCocktail(runCtx, drinkID, recipe)
			if runErr != nil {
				break
			}
		}
		phase := ""
		if snap := b.status.snapshot(); len(snap.history) > 0 {
			phase = snap.history[len(snap.history)-1].Step
		}
		b.status.finish(runErr)
		alert := orderAlert{
			drink:             displayName,
			orderID:           orderID,
			phase:             phase,
			duration:          time.Since(start),
			err:               runErr,
			operatorCancelled: isOperatorCancel(runErr),
			machineID:         b.slackMachineID(),
		}
		slackPost(context.Background(), b.logger, b.slackWebhookURL(), alertText(alert), alertBlocks(alert))
	}()

	return map[string]any{
		"drink_id":   drinkID,
		"drink":      name,
		"quantity":   quantity,
		"started_at": start.UTC().Format(time.RFC3339),
	}, nil
}

func parseMakeCocktail(raw any) (string, *Recipe, int, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", nil, 0, fmt.Errorf("make_cocktail: expected object with 'drink_id' or 'recipe', got %T", raw)
	}
	drinkID, _ := m["drink_id"].(string)
	recipeRaw, hasRecipe := m["recipe"]
	if drinkID == "" && !hasRecipe {
		return "", nil, 0, fmt.Errorf("make_cocktail: either 'drink_id' or 'recipe' is required")
	}
	quantity := 1
	if q, ok := m["quantity"].(float64); ok {
		quantity = int(q)
	}
	if quantity < 1 {
		quantity = 1
	}
	if quantity > 4 {
		quantity = 4
	}
	if !hasRecipe {
		return drinkID, nil, quantity, nil
	}
	bytes, err := json.Marshal(recipeRaw)
	if err != nil {
		return "", nil, 0, fmt.Errorf("make_cocktail: marshal recipe: %w", err)
	}
	var recipe Recipe
	if err := json.Unmarshal(bytes, &recipe); err != nil {
		return "", nil, 0, fmt.Errorf("make_cocktail: invalid recipe shape: %w", err)
	}
	return drinkID, &recipe, quantity, nil
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
