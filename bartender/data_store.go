package bartender

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

type Recipe struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Steps      []RecipeStep `json:"steps,omitempty"`
	OnMenu     bool         `json:"on_menu,omitempty"`
	Image      string       `json:"image,omitempty"`
	Section    string       `json:"section,omitempty"`
	Tag        string       `json:"tag,omitempty"`
	BaseSpirit string       `json:"base_spirit,omitempty"`
	Flavors    []string     `json:"flavors,omitempty"`
	SortOrder  int          `json:"sort_order,omitempty"`
	Pours      []Pour       `json:"pours,omitempty"`
	CrushedIce *bool        `json:"crushed_ice,omitempty"`
	Shake      *bool        `json:"shake,omitempty"`
	Strain     *bool        `json:"strain,omitempty"`
}

type RecipeStep struct {
	Kind       string  `json:"kind"`
	Ingredient string  `json:"ingredient,omitempty"`
	Oz         float64 `json:"oz,omitempty"`
	IceType    string  `json:"type,omitempty"`
	MixMode    string  `json:"mode,omitempty"`
}

func stepEnabled(flag *bool) bool {
	return flag == nil || *flag
}

func stepsFor(r Recipe) []RecipeStep {
	if len(r.Steps) > 0 {
		return r.Steps
	}
	out := make([]RecipeStep, 0, len(r.Pours)+3)
	if stepEnabled(r.CrushedIce) {
		out = append(out, RecipeStep{Kind: "ice", IceType: "crushed"})
	}
	for _, p := range r.Pours {
		out = append(out, RecipeStep{Kind: "pour", Ingredient: p.Ingredient, Oz: p.Oz})
	}
	if stepEnabled(r.Shake) {
		out = append(out, RecipeStep{Kind: "mix", MixMode: "shake"})
	}
	if stepEnabled(r.Strain) {
		out = append(out, RecipeStep{Kind: "strain"})
	}
	return out
}

type Pour struct {
	Ingredient string  `json:"ingredient"`
	Oz         float64 `json:"oz"`
}

type Inventory struct {
	Ingredients map[string]IngredientStock `json:"ingredients"`
}

type IngredientStock struct {
	InStock      bool `json:"in_stock"`
	Unopened     int  `json:"unopened,omitempty"`
	Refrigerated bool `json:"refrigerated,omitempty"`
}

type PourMenu struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Category     string      `json:"category"`
	Intro        string      `json:"intro,omitempty"`
	OnMenu       bool        `json:"on_menu,omitempty"`
	Illustration string      `json:"illustration,omitempty"`
	Sizes        []PourSize  `json:"sizes"`
	DefaultSize  string      `json:"default_size,omitempty"`
	Serves       []PourServe `json:"serves"`
	Traits       []string    `json:"traits,omitempty"`
	SortOrder    int         `json:"sort_order,omitempty"`
}

type PourSize struct {
	Label string  `json:"label"`
	Oz    float64 `json:"oz"`
}

type PourServe struct {
	Label     string `json:"label"`
	Glass     string `json:"glass,omitempty"`
	WithIce   bool   `json:"with_ice,omitempty"`
	StaffStep string `json:"staff_step,omitempty"`
}

type WhiskeyProfile struct {
	ABV     float64 `json:"abv,omitempty"`
	Region  string  `json:"region,omitempty"`
	Story   string  `json:"story,omitempty"`
	Notes   string  `json:"notes,omitempty"`
	Profile []int   `json:"profile,omitempty"`
	US      string  `json:"us,omitempty"`
	Style   string  `json:"style,omitempty"`
}

const (
	recipesFileName         = "recipes.json"
	inventoryFileName       = "inventory.json"
	slotAssignmentsFileName = "slot_assignments.json"
	pourerOzFileName        = "pourer_oz.json"
	categoriesFileName      = "categories.json"
	whiskeyProfilesFileName = "whiskey_profiles.json"
	pourMenusFileName       = "pour_menus.json"
)

var errDataStoreNotConfigured = errors.New("data_dir not configured; recipes and inventory DoCommands unavailable")

type dataStore struct {
	mu              sync.RWMutex
	dir             string
	recipes         []Recipe
	inventory       Inventory
	slotAssignments map[string]string
	pourerOz        map[string]float64
	categories      map[string]string
	whiskeyProfiles map[string]WhiskeyProfile
	pourMenus       []PourMenu
}

func newDataStore(dir string) (*dataStore, error) {
	if dir == "" {
		return nil, errDataStoreNotConfigured
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data_dir %q: %w", dir, err)
	}
	ds := &dataStore{
		dir:             dir,
		inventory:       Inventory{Ingredients: map[string]IngredientStock{}},
		slotAssignments: map[string]string{},
		pourerOz:        map[string]float64{},
		categories:      map[string]string{},
		whiskeyProfiles: map[string]WhiskeyProfile{},
	}
	if err := ds.loadRecipes(); err != nil {
		return nil, err
	}
	if err := ds.loadInventory(); err != nil {
		return nil, err
	}
	if err := ds.loadSlotAssignments(); err != nil {
		return nil, err
	}
	if err := ds.loadPourerOz(); err != nil {
		return nil, err
	}
	if err := ds.loadCategories(); err != nil {
		return nil, err
	}
	if err := ds.loadWhiskeyProfiles(); err != nil {
		return nil, err
	}
	if err := ds.loadPourMenus(); err != nil {
		return nil, err
	}
	return ds, nil
}

func (ds *dataStore) loadRecipes() error {
	path := filepath.Join(ds.dir, recipesFileName)
	bytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var recipes []Recipe
	if err := json.Unmarshal(bytes, &recipes); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	ds.recipes = recipes
	return nil
}

func (ds *dataStore) loadInventory() error {
	path := filepath.Join(ds.dir, inventoryFileName)
	bytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var inv Inventory
	if err := json.Unmarshal(bytes, &inv); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if inv.Ingredients == nil {
		inv.Ingredients = map[string]IngredientStock{}
	}
	ds.inventory = inv
	return nil
}

func (ds *dataStore) Recipes() []Recipe {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	out := make([]Recipe, len(ds.recipes))
	copy(out, ds.recipes)
	return out
}

func (ds *dataStore) GetInventory() Inventory {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	out := Inventory{Ingredients: make(map[string]IngredientStock, len(ds.inventory.Ingredients))}
	for k, v := range ds.inventory.Ingredients {
		out.Ingredients[k] = v
	}
	return out
}

func (ds *dataStore) UpdateRecipes(recipes []Recipe) error {
	if recipes == nil {
		recipes = []Recipe{}
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.recipes = recipes
	return ds.persistRecipesLocked()
}

func (ds *dataStore) persistRecipesLocked() error {
	bytes, err := json.MarshalIndent(ds.recipes, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal recipes: %w", err)
	}
	path := filepath.Join(ds.dir, recipesFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

func (ds *dataStore) UpdateInventoryItem(ingredient string, inStock bool) error {
	if ingredient == "" {
		return errors.New("ingredient is required")
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if err := ds.checkIngredientNameLocked(ingredient); err != nil {
		return err
	}
	stock := ds.inventory.Ingredients[ingredient]
	stock.InStock = inStock
	ds.inventory.Ingredients[ingredient] = stock
	return ds.persistInventoryLocked()
}

const maxUnopened = 9999

func (ds *dataStore) AdjustUnopened(ingredient string, delta int) (int, error) {
	if delta < -maxUnopened || delta > maxUnopened {
		return 0, fmt.Errorf("unopened delta must be between %d and %d, got %d", -maxUnopened, maxUnopened, delta)
	}
	stock, err := ds.updateKnownStock(ingredient, func(s *IngredientStock) {
		s.Unopened = min(max(s.Unopened+delta, 0), maxUnopened)
	})
	return stock.Unopened, err
}

func (ds *dataStore) SetUnopened(ingredient string, count int) error {
	if count < 0 || count > maxUnopened {
		return fmt.Errorf("unopened count must be between 0 and %d, got %d", maxUnopened, count)
	}
	_, err := ds.updateKnownStock(ingredient, func(s *IngredientStock) { s.Unopened = count })
	return err
}

func (ds *dataStore) SetRefrigerated(ingredient string, refrigerated bool) error {
	_, err := ds.updateKnownStock(ingredient, func(s *IngredientStock) { s.Refrigerated = refrigerated })
	return err
}

func (ds *dataStore) updateKnownStock(ingredient string, update func(*IngredientStock)) (IngredientStock, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	stock, err := ds.knownStockLocked(ingredient)
	if err != nil {
		return IngredientStock{}, err
	}
	update(&stock)
	ds.inventory.Ingredients[ingredient] = stock
	return stock, ds.persistInventoryLocked()
}

// Slot-only ingredients have no inventory entry yet but are still real ingredients.
func (ds *dataStore) knownStockLocked(ingredient string) (IngredientStock, error) {
	if ingredient == "" {
		return IngredientStock{}, errors.New("ingredient is required")
	}
	if stock, ok := ds.inventory.Ingredients[ingredient]; ok {
		return stock, nil
	}
	for _, assigned := range ds.slotAssignments {
		if assigned == ingredient {
			return IngredientStock{}, nil
		}
	}
	return IngredientStock{}, fmt.Errorf("unknown ingredient %q", ingredient)
}

// Rejects a new spelling of an ingredient that's already stored under a name that displays the same.
func (ds *dataStore) checkIngredientNameLocked(ingredient string) error {
	want := normalizeIngredientName(ingredient)
	var match string
	for _, name := range ds.ingredientNamesLocked() {
		if name == ingredient {
			return nil
		}
		if match == "" && normalizeIngredientName(name) == want {
			match = name
		}
	}
	if match != "" {
		return fmt.Errorf("ingredient %q already exists as %q", ingredient, match)
	}
	return nil
}

func (ds *dataStore) ingredientNamesLocked() []string {
	names := make([]string, 0, len(ds.inventory.Ingredients)+len(ds.slotAssignments)+len(ds.pourerOz)+len(ds.categories)+len(ds.whiskeyProfiles))
	for name := range ds.inventory.Ingredients {
		names = append(names, name)
	}
	for _, name := range ds.slotAssignments {
		names = append(names, name)
	}
	for name := range ds.pourerOz {
		names = append(names, name)
	}
	for name := range ds.categories {
		names = append(names, name)
	}
	for name := range ds.whiskeyProfiles {
		names = append(names, name)
	}
	return names
}

// Matches the web app's titleCase: names that display the same are the same ingredient.
func normalizeIngredientName(name string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return r == '-' || r == '_' || unicode.IsSpace(r)
	}), " ")
}

// Clears every record keyed by the ingredient: a leftover slot would keep it as a slot-only ingredient, and
// leftover pourer, category or whiskey-profile entries are invisible but still block re-adding a similar name.
func (ds *dataStore) DeleteInventoryItem(ingredient string) error {
	if ingredient == "" {
		return errors.New("ingredient is required")
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	slotsChanged := false
	for slot, assigned := range ds.slotAssignments {
		if assigned == ingredient {
			delete(ds.slotAssignments, slot)
			slotsChanged = true
		}
	}
	if slotsChanged {
		if err := ds.persistSlotAssignmentsLocked(); err != nil {
			return err
		}
	}
	if _, ok := ds.pourerOz[ingredient]; ok {
		delete(ds.pourerOz, ingredient)
		if err := ds.persistPourerOzLocked(); err != nil {
			return err
		}
	}
	if _, ok := ds.categories[ingredient]; ok {
		delete(ds.categories, ingredient)
		if err := ds.persistCategoriesLocked(); err != nil {
			return err
		}
	}
	if _, ok := ds.whiskeyProfiles[ingredient]; ok {
		delete(ds.whiskeyProfiles, ingredient)
		if err := ds.persistWhiskeyProfilesLocked(); err != nil {
			return err
		}
	}
	delete(ds.inventory.Ingredients, ingredient)
	return ds.persistInventoryLocked()
}

func (ds *dataStore) loadSlotAssignments() error {
	path := filepath.Join(ds.dir, slotAssignmentsFileName)
	bytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var sa map[string]string
	if err := json.Unmarshal(bytes, &sa); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if sa == nil {
		sa = map[string]string{}
	}
	ds.slotAssignments = sa
	return nil
}

func (ds *dataStore) SlotAssignments() map[string]string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	out := make(map[string]string, len(ds.slotAssignments))
	for k, v := range ds.slotAssignments {
		out[k] = v
	}
	return out
}

func (ds *dataStore) UpdateSlotAssignment(slot, ingredient string) error {
	if slot == "" {
		return errors.New("slot is required")
	}
	if ingredient == "" {
		return errors.New("ingredient is required; call clear_slot_assignment to empty a slot")
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if err := ds.checkIngredientNameLocked(ingredient); err != nil {
		return err
	}
	for existing, ing := range ds.slotAssignments {
		if ing == ingredient && existing != slot {
			return fmt.Errorf("ingredient %q is already assigned to slot %q", ingredient, existing)
		}
	}
	ds.slotAssignments[slot] = ingredient
	return ds.persistSlotAssignmentsLocked()
}

func (ds *dataStore) ClearSlotAssignment(slot string) error {
	if slot == "" {
		return errors.New("slot is required")
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.slotAssignments, slot)
	return ds.persistSlotAssignmentsLocked()
}

func (ds *dataStore) persistSlotAssignmentsLocked() error {
	bytes, err := json.MarshalIndent(ds.slotAssignments, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal slot_assignments: %w", err)
	}
	path := filepath.Join(ds.dir, slotAssignmentsFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

func (ds *dataStore) loadPourerOz() error {
	path := filepath.Join(ds.dir, pourerOzFileName)
	bytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var p map[string]float64
	if err := json.Unmarshal(bytes, &p); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if p == nil {
		p = map[string]float64{}
	}
	ds.pourerOz = p
	return nil
}

func (ds *dataStore) PourerOz() map[string]float64 {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	out := make(map[string]float64, len(ds.pourerOz))
	for k, v := range ds.pourerOz {
		out[k] = v
	}
	return out
}

func (ds *dataStore) UpdatePourerOz(ingredient string, oz float64) error {
	if ingredient == "" {
		return errors.New("ingredient is required")
	}
	if oz <= 0 {
		return fmt.Errorf("oz must be > 0, got %v", oz)
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if err := ds.checkIngredientNameLocked(ingredient); err != nil {
		return err
	}
	ds.pourerOz[ingredient] = oz
	return ds.persistPourerOzLocked()
}

func (ds *dataStore) persistPourerOzLocked() error {
	bytes, err := json.MarshalIndent(ds.pourerOz, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pourer_oz: %w", err)
	}
	path := filepath.Join(ds.dir, pourerOzFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

func (ds *dataStore) loadCategories() error {
	path := filepath.Join(ds.dir, categoriesFileName)
	bytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var c map[string]string
	if err := json.Unmarshal(bytes, &c); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if c == nil {
		c = map[string]string{}
	}
	ds.categories = c
	return nil
}

func (ds *dataStore) Categories() map[string]string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	out := make(map[string]string, len(ds.categories))
	for k, v := range ds.categories {
		out[k] = v
	}
	return out
}

func (ds *dataStore) UpdateCategory(ingredient, category string) error {
	if ingredient == "" {
		return errors.New("ingredient is required")
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if category == "" {
		delete(ds.categories, ingredient)
	} else {
		if err := ds.checkIngredientNameLocked(ingredient); err != nil {
			return err
		}
		ds.categories[ingredient] = category
	}
	return ds.persistCategoriesLocked()
}

func (ds *dataStore) persistCategoriesLocked() error {
	bytes, err := json.MarshalIndent(ds.categories, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal categories: %w", err)
	}
	path := filepath.Join(ds.dir, categoriesFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

func (ds *dataStore) loadWhiskeyProfiles() error {
	path := filepath.Join(ds.dir, whiskeyProfilesFileName)
	bytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var p map[string]WhiskeyProfile
	if err := json.Unmarshal(bytes, &p); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if p == nil {
		p = map[string]WhiskeyProfile{}
	}
	ds.whiskeyProfiles = p
	return nil
}

func (ds *dataStore) WhiskeyProfiles() map[string]WhiskeyProfile {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	out := make(map[string]WhiskeyProfile, len(ds.whiskeyProfiles))
	for k, v := range ds.whiskeyProfiles {
		out[k] = v
	}
	return out
}

func (ds *dataStore) UpdateWhiskeyProfile(ingredient string, profile WhiskeyProfile) error {
	if ingredient == "" {
		return errors.New("ingredient is required")
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if err := ds.checkIngredientNameLocked(ingredient); err != nil {
		return err
	}
	ds.whiskeyProfiles[ingredient] = profile
	return ds.persistWhiskeyProfilesLocked()
}

func (ds *dataStore) ClearWhiskeyProfile(ingredient string) error {
	if ingredient == "" {
		return errors.New("ingredient is required")
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.whiskeyProfiles, ingredient)
	return ds.persistWhiskeyProfilesLocked()
}

func (ds *dataStore) persistWhiskeyProfilesLocked() error {
	bytes, err := json.MarshalIndent(ds.whiskeyProfiles, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal whiskey_profiles: %w", err)
	}
	path := filepath.Join(ds.dir, whiskeyProfilesFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

func defaultWhiskeyBarMenu() PourMenu {
	return PourMenu{
		ID:       "whiskey-bar",
		Name:     "Whiskey bar",
		Category: "Whiskey",
		OnMenu:   true,
		Sizes: []PourSize{
			{Label: "Taste", Oz: 1},
			{Label: "Pour", Oz: 2},
		},
		DefaultSize: "Taste",
		Serves: []PourServe{
			{Label: "Neat", Glass: "rocks"},
			{Label: "On the rocks", Glass: "rocks", WithIce: true},
		},
		Traits: []string{"Peat", "Smoke", "Smoothness", "Sweetness", "Fruitiness", "Heat", "Body"},
	}
}

func (ds *dataStore) loadPourMenus() error {
	path := filepath.Join(ds.dir, pourMenusFileName)
	bytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		ds.pourMenus = []PourMenu{defaultWhiskeyBarMenu()}
		return ds.persistPourMenusLocked()
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var menus []PourMenu
	if err := json.Unmarshal(bytes, &menus); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	ds.pourMenus = menus
	return nil
}

func (ds *dataStore) PourMenus() []PourMenu {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	out := make([]PourMenu, len(ds.pourMenus))
	copy(out, ds.pourMenus)
	return out
}

func (ds *dataStore) UpdatePourMenus(menus []PourMenu) error {
	if menus == nil {
		menus = []PourMenu{}
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.pourMenus = menus
	return ds.persistPourMenusLocked()
}

func (ds *dataStore) persistPourMenusLocked() error {
	bytes, err := json.MarshalIndent(ds.pourMenus, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pour_menus: %w", err)
	}
	path := filepath.Join(ds.dir, pourMenusFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

func (ds *dataStore) persistInventoryLocked() error {
	bytes, err := json.MarshalIndent(ds.inventory, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal inventory: %w", err)
	}
	path := filepath.Join(ds.dir, inventoryFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
