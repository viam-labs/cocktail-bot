package bartender

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Recipe struct {
	ID    string       `json:"id"`
	Name  string       `json:"name"`
	Steps []RecipeStep `json:"steps"`
}

type RecipeStep struct {
	Verb         string  `json:"verb"`
	Bottle       string  `json:"bottle,omitempty"`
	Station      string  `json:"station,omitempty"`
	Source       string  `json:"source,omitempty"`
	StrainFlow   string  `json:"strain_flow,omitempty"`
	Oz           float64 `json:"oz,omitempty"`
	PourMs       int     `json:"pour_ms,omitempty"`
	DwellMs      int     `json:"dwell_ms,omitempty"`
	DrainDwellMs int     `json:"drain_dwell_ms,omitempty"`
	DumpDwellMs  int     `json:"dump_dwell_ms,omitempty"`
}

type Inventory struct {
	Ingredients map[string]IngredientStock `json:"ingredients"`
}

type IngredientStock struct {
	InStock bool `json:"in_stock"`
}

const (
	recipesFileName   = "recipes.json"
	inventoryFileName = "inventory.json"
)

var errDataStoreNotConfigured = errors.New("data_dir not configured; recipes and inventory DoCommands unavailable")

type dataStore struct {
	mu        sync.RWMutex
	dir       string
	recipes   []Recipe
	inventory Inventory
}

func newDataStore(dir string) (*dataStore, error) {
	if dir == "" {
		return nil, errDataStoreNotConfigured
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data_dir %q: %w", dir, err)
	}
	ds := &dataStore{dir: dir, inventory: Inventory{Ingredients: map[string]IngredientStock{}}}
	if err := ds.loadRecipes(); err != nil {
		return nil, err
	}
	if err := ds.loadInventory(); err != nil {
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
	ds.inventory.Ingredients[ingredient] = IngredientStock{InStock: inStock}
	return ds.persistInventoryLocked()
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
