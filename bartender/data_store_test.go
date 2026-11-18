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

func TestSetRefrigeratedPersistsAcrossInStockToggles(t *testing.T) {
	dir := t.TempDir()
	ds, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateInventoryItem("vermouth", true), test.ShouldBeNil)

	test.That(t, ds.SetRefrigerated("vermouth", true), test.ShouldBeNil)
	test.That(t, ds.UpdateInventoryItem("vermouth", false), test.ShouldBeNil)

	ds2, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds2.GetInventory().Ingredients["vermouth"], test.ShouldResemble, IngredientStock{InStock: false, Refrigerated: true})
}

func TestSetRefrigeratedForSlotOnlyIngredient(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateSlotAssignment("slot-a-front", "gin"), test.ShouldBeNil)

	test.That(t, ds.SetRefrigerated("gin", true), test.ShouldBeNil)
	test.That(t, ds.GetInventory().Ingredients["gin"], test.ShouldResemble, IngredientStock{Refrigerated: true})
}

func TestSetRefrigeratedRejectsUnknownIngredient(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)

	test.That(t, ds.SetRefrigerated("rum", true), test.ShouldNotBeNil)
	test.That(t, ds.SetRefrigerated("", true), test.ShouldNotBeNil)
	test.That(t, len(ds.GetInventory().Ingredients), test.ShouldEqual, 0)
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
