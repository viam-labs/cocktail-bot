package bartender

import (
	"context"
	"encoding/json"
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
	if raw, ok := cmd["mix"]; ok {
		return b.handleMix(ctx, raw)
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
	if _, ok := cmd["get_inventory"]; ok {
		return b.handleGetInventory()
	}
	if raw, ok := cmd["update_inventory_item"]; ok {
		return b.handleUpdateInventoryItem(raw)
	}
	if raw, ok := cmd["update_recipes"]; ok {
		return b.handleUpdateRecipes(raw)
	}
	if raw, ok := cmd["make_cocktail"]; ok {
		return b.handleMakeCocktail(ctx, raw)
	}
	return nil, fmt.Errorf("unknown command, supported: prepare_order, get_queue, cancel, execute_action, pickup_pour_return, dispense_ice, pour_into_glasses, find_glass, find_and_pour, find_and_pour_from_shaker, mix, pour_from_shaker, pour_into_shaker, strain_shaker, rotate_shakers, get_recipes, get_inventory, update_inventory_item, update_recipes, make_cocktail")
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

// handleCancel aborts whatever verb is running by cancelling its context. The
// arm stops wherever it is; held/gripper state is left untouched, so the
// operator can inspect, intervene, and re-run from a known state. If nothing
// is running, returns {cancelled: false}.
func (b *bartender) handleCancel() (map[string]any, error) {
	cancelled := b.cancelRunning()
	return map[string]any{"cancelled": cancelled}, nil
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

func (b *bartender) handleGetInventory() (map[string]any, error) {
	if b.dataStore == nil {
		return nil, errDataStoreNotConfigured
	}
	return map[string]any{"inventory": b.dataStore.GetInventory()}, nil
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

func (b *bartender) handleMakeCocktail(ctx context.Context, raw any) (map[string]any, error) {
	drinkID, recipe, err := parseMakeCocktail(raw)
	if err != nil {
		return nil, err
	}
	ctx = ctxWithOrderID(ctx, "manual-"+time.Now().UTC().Format("20060102_150405"))
	ctx, done := b.withCancel(ctx)
	defer done()
	start := time.Now()
	runErr := b.makeCocktail(ctx, drinkID, recipe)
	name := drinkID
	if recipe != nil && recipe.Name != "" {
		name = recipe.Name
	}
	slackPost(context.Background(), b.logger, b.slackWebhookURL(), formatOrderAlert(name, time.Since(start), runErr, ""))
	if runErr != nil {
		return nil, runErr
	}
	return map[string]any{
		"drink_id":    drinkID,
		"duration_ms": time.Since(start).Milliseconds(),
	}, nil
}

func parseMakeCocktail(raw any) (string, *Recipe, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("make_cocktail: expected object with 'drink_id' or 'recipe', got %T", raw)
	}
	drinkID, _ := m["drink_id"].(string)
	recipeRaw, hasRecipe := m["recipe"]
	if drinkID == "" && !hasRecipe {
		return "", nil, fmt.Errorf("make_cocktail: either 'drink_id' or 'recipe' is required")
	}
	if !hasRecipe {
		return drinkID, nil, nil
	}
	bytes, err := json.Marshal(recipeRaw)
	if err != nil {
		return "", nil, fmt.Errorf("make_cocktail: marshal recipe: %w", err)
	}
	var recipe Recipe
	if err := json.Unmarshal(bytes, &recipe); err != nil {
		return "", nil, fmt.Errorf("make_cocktail: invalid recipe shape: %w", err)
	}
	return drinkID, &recipe, nil
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
