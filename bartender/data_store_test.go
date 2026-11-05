package bartender

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

func TestNewDataStoreRequiresDir(t *testing.T) {
	_, err := newDataStore("")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "data_dir")
}

func TestNewDataStoreCreatesMissingDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nested", "data")
	ds, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds, test.ShouldNotBeNil)
	st, err := os.Stat(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, st.IsDir(), test.ShouldBeTrue)
}

func TestLoadRecipesFromDisk(t *testing.T) {
	dir := t.TempDir()
	recipes := []Recipe{
		{
			ID:   "espresso_martini",
			Name: "Espresso Martini",
			Pours: []Pour{
				{Ingredient: "vodka", Oz: 2.0},
				{Ingredient: "coffee-liquor", Oz: 1.0},
			},
		},
	}
	writeJSON(t, filepath.Join(dir, recipesFileName), recipes)

	ds, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	got := ds.Recipes()
	test.That(t, len(got), test.ShouldEqual, 1)
	test.That(t, got[0].ID, test.ShouldEqual, "espresso_martini")
	test.That(t, len(got[0].Pours), test.ShouldEqual, 2)
}

func TestLoadInventoryFromDisk(t *testing.T) {
	dir := t.TempDir()
	inv := Inventory{Ingredients: map[string]IngredientStock{
		"vodka":         {InStock: true},
		"coffee-liquor": {InStock: false},
	}}
	writeJSON(t, filepath.Join(dir, inventoryFileName), inv)

	ds, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	got := ds.GetInventory()
	test.That(t, got.Ingredients["vodka"].InStock, test.ShouldBeTrue)
	test.That(t, got.Ingredients["coffee-liquor"].InStock, test.ShouldBeFalse)
}

func TestEmptyDirYieldsEmptyData(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(ds.Recipes()), test.ShouldEqual, 0)
	test.That(t, len(ds.GetInventory().Ingredients), test.ShouldEqual, 0)
}

func TestUpdateInventoryItemPersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	ds, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)

	err = ds.UpdateInventoryItem("vodka", true)
	test.That(t, err, test.ShouldBeNil)

	ds2, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds2.GetInventory().Ingredients["vodka"].InStock, test.ShouldBeTrue)

	err = ds.UpdateInventoryItem("vodka", false)
	test.That(t, err, test.ShouldBeNil)

	ds3, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds3.GetInventory().Ingredients["vodka"].InStock, test.ShouldBeFalse)
}

func TestUpdateInventoryItemRejectsEmptyIngredient(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	err = ds.UpdateInventoryItem("", true)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestUpdateInventoryItemKeepsUnopenedCount(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateInventoryItem("vodka", true), test.ShouldBeNil)
	test.That(t, ds.SetUnopened("vodka", 3), test.ShouldBeNil)

	test.That(t, ds.UpdateInventoryItem("vodka", false), test.ShouldBeNil)
	test.That(t, ds.GetInventory().Ingredients["vodka"].Unopened, test.ShouldEqual, 3)
}

func TestUpdateInventoryItemRejectsEquivalentName(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateInventoryItem("coffee-liquor", true), test.ShouldBeNil)

	for _, name := range []string{"Coffee Liquor", "coffee_liquor", " coffee  liquor "} {
		err := ds.UpdateInventoryItem(name, true)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `already exists as "coffee-liquor"`)
	}
	test.That(t, len(ds.GetInventory().Ingredients), test.ShouldEqual, 1)
}

func TestUpdateInventoryItemRejectsNameMatchingSlotOnlyIngredient(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateSlotAssignment("slot-a-front", "gin"), test.ShouldBeNil)

	err = ds.UpdateInventoryItem("Gin", true)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, ds.UpdateInventoryItem("gin", true), test.ShouldBeNil)
}

func TestAdjustUnopenedPersistsAndClampsAtZero(t *testing.T) {
	dir := t.TempDir()
	ds, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateInventoryItem("vodka", true), test.ShouldBeNil)

	count, err := ds.AdjustUnopened("vodka", 2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, count, test.ShouldEqual, 2)
	count, err = ds.AdjustUnopened("vodka", -1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, count, test.ShouldEqual, 1)

	ds2, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds2.GetInventory().Ingredients["vodka"].Unopened, test.ShouldEqual, 1)

	count, err = ds.AdjustUnopened("vodka", -5)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, count, test.ShouldEqual, 0)
	test.That(t, ds.GetInventory().Ingredients["vodka"].InStock, test.ShouldBeTrue)
}

func TestUnopenedForSlotOnlyIngredient(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateSlotAssignment("slot-a-front", "gin"), test.ShouldBeNil)

	count, err := ds.AdjustUnopened("gin", 1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, count, test.ShouldEqual, 1)
	test.That(t, ds.GetInventory().Ingredients["gin"].InStock, test.ShouldBeFalse)
}

func TestUnopenedRejectsUnknownIngredient(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)

	_, err = ds.AdjustUnopened("rum", 1)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, ds.SetUnopened("rum", 1), test.ShouldNotBeNil)
	test.That(t, ds.SetUnopened("", 1), test.ShouldNotBeNil)
	test.That(t, len(ds.GetInventory().Ingredients), test.ShouldEqual, 0)
}

func TestSetUnopenedRejectsNegative(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateInventoryItem("vodka", true), test.ShouldBeNil)
	test.That(t, ds.SetUnopened("vodka", -1), test.ShouldNotBeNil)
}

func TestSetRefrigeratedKeepsOtherFields(t *testing.T) {
	dir := t.TempDir()
	ds, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateInventoryItem("vermouth", true), test.ShouldBeNil)
	test.That(t, ds.SetUnopened("vermouth", 2), test.ShouldBeNil)

	test.That(t, ds.SetRefrigerated("vermouth", true), test.ShouldBeNil)
	test.That(t, ds.UpdateInventoryItem("vermouth", false), test.ShouldBeNil)

	ds2, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	got := ds2.GetInventory().Ingredients["vermouth"]
	test.That(t, got, test.ShouldResemble, IngredientStock{InStock: false, Unopened: 2, Refrigerated: true})
	test.That(t, ds.SetRefrigerated("rum", true), test.ShouldNotBeNil)
}

func TestOtherWritesRejectEquivalentName(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateInventoryItem("coffee-liquor", true), test.ShouldBeNil)

	test.That(t, ds.UpdateSlotAssignment("slot-a-front", "Coffee Liquor"), test.ShouldNotBeNil)
	test.That(t, ds.UpdatePourerOz("Coffee Liquor", 1), test.ShouldNotBeNil)
	test.That(t, ds.UpdateCategory("Coffee Liquor", "liqueur"), test.ShouldNotBeNil)
	test.That(t, ds.UpdateWhiskeyProfile("Coffee Liquor", WhiskeyProfile{}), test.ShouldNotBeNil)
	test.That(t, ds.UpdateSlotAssignment("slot-a-front", "coffee-liquor"), test.ShouldBeNil)

	test.That(t, ds.UpdateCategory("simple-syrup", "syrup"), test.ShouldBeNil)
	test.That(t, ds.UpdateInventoryItem("Simple Syrup", true), test.ShouldNotBeNil)
}

func TestUnopenedBounds(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateInventoryItem("vodka", true), test.ShouldBeNil)

	test.That(t, ds.SetUnopened("vodka", maxUnopened+1), test.ShouldNotBeNil)
	_, err = ds.AdjustUnopened("vodka", maxUnopened+1)
	test.That(t, err, test.ShouldNotBeNil)

	test.That(t, ds.SetUnopened("vodka", maxUnopened), test.ShouldBeNil)
	count, err := ds.AdjustUnopened("vodka", 1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, count, test.ShouldEqual, maxUnopened)
}

func TestDeleteInventoryItemClearsSlot(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateSlotAssignment("slot-a-front", "vermouth"), test.ShouldBeNil)
	test.That(t, ds.SetUnopened("vermouth", 2), test.ShouldBeNil)

	test.That(t, ds.DeleteInventoryItem("vermouth"), test.ShouldBeNil)
	test.That(t, len(ds.SlotAssignments()), test.ShouldEqual, 0)
	_, err = ds.AdjustUnopened("vermouth", 1)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestParseIngredientInt(t *testing.T) {
	ingredient, n, err := parseIngredientInt("adjust_unopened", "delta", map[string]any{"ingredient": "vodka", "delta": -1.0})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ingredient, test.ShouldEqual, "vodka")
	test.That(t, n, test.ShouldEqual, -1)

	for _, raw := range []any{
		"vodka",
		map[string]any{"delta": 1.0},
		map[string]any{"ingredient": "vodka"},
		map[string]any{"ingredient": "vodka", "delta": 1.5},
		map[string]any{"ingredient": "vodka", "delta": "1"},
		map[string]any{"ingredient": "vodka", "delta": 1e20},
	} {
		_, _, err := parseIngredientInt("adjust_unopened", "delta", raw)
		test.That(t, err, test.ShouldNotBeNil)
	}
}

func TestGetInventoryReturnsCopy(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	err = ds.UpdateInventoryItem("vodka", true)
	test.That(t, err, test.ShouldBeNil)

	got := ds.GetInventory()
	got.Ingredients["vodka"] = IngredientStock{InStock: false}
	fresh := ds.GetInventory()
	test.That(t, fresh.Ingredients["vodka"].InStock, test.ShouldBeTrue)
}

func TestPourMenusSeededOnFreshDir(t *testing.T) {
	dir := t.TempDir()
	ds, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	menus := ds.PourMenus()
	test.That(t, len(menus), test.ShouldEqual, 1)
	test.That(t, menus[0].ID, test.ShouldEqual, "whiskey-bar")
	test.That(t, menus[0].Category, test.ShouldEqual, "Whiskey")

	ds2, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(ds2.PourMenus()), test.ShouldEqual, 1)
}

func TestUpdatePourMenusPersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	ds, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)

	err = ds.UpdatePourMenus([]PourMenu{
		{
			ID:       "wine",
			Name:     "Wine",
			Category: "Wine",
			OnMenu:   true,
			Sizes:    []PourSize{{Label: "Taste", Oz: 2}, {Label: "Glass", Oz: 5}},
			Serves:   []PourServe{{Label: "Serve", Glass: "stemless"}},
			Traits:   []string{"Body", "Sweetness", "Acidity", "Tannin", "Fruit"},
		},
	})
	test.That(t, err, test.ShouldBeNil)

	ds2, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	menus := ds2.PourMenus()
	test.That(t, len(menus), test.ShouldEqual, 1)
	test.That(t, menus[0].ID, test.ShouldEqual, "wine")
	test.That(t, len(menus[0].Sizes), test.ShouldEqual, 2)
	test.That(t, menus[0].Serves[0].Glass, test.ShouldEqual, "stemless")
}

func TestDataDirFallsBackToModuleEnv(t *testing.T) {
	t.Setenv("VIAM_MODULE_DATA", "/env/dir")

	explicit := &Config{DataDir: "/explicit"}
	test.That(t, explicit.dataDir(), test.ShouldEqual, "/explicit")

	fallback := &Config{}
	test.That(t, fallback.dataDir(), test.ShouldEqual, "/env/dir")

	t.Setenv("VIAM_MODULE_DATA", "")
	unset := &Config{}
	test.That(t, unset.dataDir(), test.ShouldEqual, "")
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	bytes, err := json.Marshal(v)
	test.That(t, err, test.ShouldBeNil)
	err = os.WriteFile(path, bytes, 0o644)
	test.That(t, err, test.ShouldBeNil)
}
