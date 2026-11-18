package bartender

import (
	"context"
	"encoding/json"
	"testing"

	"go.viam.com/test"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

func testGlasses() []Glass {
	return []Glass{
		{ID: "martini", Name: "Martini", CapacityOz: 7, Image: "martini", PoseSwitcher: "serve-martini", Obstacle: "martini-glass"},
		{ID: "highball", Name: "Highball", CapacityOz: 12, Image: "highball", PoseSwitcher: "serve-highball"},
	}
}

func TestConfigValidatesGlasses(t *testing.T) {
	cfg := Config{
		ArmName:           "arm",
		GripperName:       "gripper",
		PoseSwitcherNames: []string{"serve-martini", "serve-highball"},
		Glasses:           testGlasses(),
	}
	_, _, err := cfg.Validate("bartender")
	test.That(t, err, test.ShouldBeNil)

	cfg.Glasses = []Glass{
		{ID: "a", Name: "A", CapacityOz: 5, PoseSwitcher: "serve-martini"},
		{ID: "a", Name: "", CapacityOz: 0, Image: "boot", PoseSwitcher: "nowhere"},
		{ID: "b", Name: "B", CapacityOz: 5},
	}
	_, _, err = cfg.Validate("bartender")
	test.That(t, err, test.ShouldNotBeNil)
	for _, want := range []string{
		"used twice", "name is required", "capacity_oz", `image "boot"`,
		`pose_switcher "nowhere" is not in pose_switcher_names`, "glasses[2]: pose_switcher is required",
	} {
		test.That(t, err.Error(), test.ShouldContainSubstring, want)
	}
}

func TestServingGlass(t *testing.T) {
	glasses := testGlasses()
	r := Recipe{Glass: "martini", DoubleGlass: "highball"}
	g, err := servingGlass(r, false, glasses)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g.PoseSwitcher, test.ShouldEqual, "serve-martini")
	g, err = servingGlass(r, true, glasses)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g.Name, test.ShouldEqual, "Highball")
	g, err = servingGlass(Recipe{}, false, glasses)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g, test.ShouldBeNil)
	_, err = servingGlass(Recipe{Glass: "tiki"}, false, glasses)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestUpdateRecipesRefusesUnknownGlass(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	b := &bartender{cfg: &Config{Glasses: testGlasses()}, dataStore: ds}
	_, err = b.handleUpdateRecipes(map[string]any{"recipes": []any{
		map[string]any{"id": "a", "name": "A", "glass": "martini", "double_glass": "boot"},
	}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `recipe "a" uses double glass "boot"`)
	test.That(t, len(ds.Recipes()), test.ShouldEqual, 0)
}

func TestMakeCocktailPutsGlassOnOrder(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	// No pours, so step validation doesn't need bottle switches.
	r := Recipe{ID: "iced", Name: "Iced", Steps: []RecipeStep{{Kind: "ice"}, {Kind: "mix"}}, Glass: "martini", DoubleGlass: "highball"}
	test.That(t, ds.UpdateRecipes([]Recipe{r}), test.ShouldBeNil)
	b := &bartender{cfg: &Config{RecipeDefaults: &RecipeDefaults{}, Glasses: testGlasses()}, dataStore: ds, queue: order.NewQueue()}

	resp, err := b.handleMakeCocktail(context.Background(), map[string]any{"drink_id": "iced", "double": true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["glass"], test.ShouldEqual, "Highball")
	queued := b.queue.State().Queue[0]
	test.That(t, queued.Glass, test.ShouldEqual, "Highball")
	var job cocktailJob
	test.That(t, json.Unmarshal(queued.Payload, &job), test.ShouldBeNil)
	test.That(t, job.Glass.PoseSwitcher, test.ShouldEqual, "serve-highball")
	test.That(t, job.ServePhase, test.ShouldEqual, "Serving into the highball glass")
}

func TestMakeCocktailRefusesGlassMissingFromConfig(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	r := Recipe{ID: "iced", Name: "Iced", Steps: []RecipeStep{{Kind: "ice"}}, Glass: "tiki"}
	test.That(t, ds.UpdateRecipes([]Recipe{r}), test.ShouldBeNil)
	b := &bartender{cfg: &Config{RecipeDefaults: &RecipeDefaults{}, Glasses: testGlasses()}, dataStore: ds, queue: order.NewQueue()}

	_, err = b.handleMakeCocktail(context.Background(), map[string]any{"drink_id": "iced"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `glass "tiki"`)
	test.That(t, len(b.queue.State().Queue), test.ShouldEqual, 0)
}

func TestGetGlassesFromConfig(t *testing.T) {
	b := &bartender{cfg: &Config{Glasses: testGlasses()}}
	resp, err := b.handleGetGlasses()
	test.That(t, err, test.ShouldBeNil)
	first := resp["glasses"].([]any)[0].(map[string]any)
	test.That(t, first["id"], test.ShouldEqual, "martini")
	test.That(t, first["capacity_oz"], test.ShouldEqual, 7.0)
	test.That(t, first["image"], test.ShouldEqual, "martini")

	empty, err := (&bartender{cfg: &Config{}}).handleGetGlasses()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, empty["glasses"], test.ShouldResemble, []any{})
}
