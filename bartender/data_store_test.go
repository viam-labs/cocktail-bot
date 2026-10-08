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
			Steps: []RecipeStep{
				{Verb: "pour_into_shaker", Bottle: "vodka", Oz: 2.0},
				{Verb: "dispense_ice", Station: "ice-station", DwellMs: 3000},
				{Verb: "mix", Station: "mixer", DwellMs: 10000},
				{Verb: "pour_from_shaker", Station: "serving", PourMs: 5000},
			},
		},
	}
	writeJSON(t, filepath.Join(dir, recipesFileName), recipes)

	ds, err := newDataStore(dir)
	test.That(t, err, test.ShouldBeNil)
	got := ds.Recipes()
	test.That(t, len(got), test.ShouldEqual, 1)
	test.That(t, got[0].ID, test.ShouldEqual, "espresso_martini")
	test.That(t, len(got[0].Steps), test.ShouldEqual, 4)
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
