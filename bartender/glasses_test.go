package bartender

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/gripper"
	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
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
	_, err = servingGlass(Recipe{Glass: "martini"}, true, glasses)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "double_glass")
}

func TestUpdateRecipesRequiresDoubleGlassForDoublableDrinks(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	b := &bartender{cfg: &Config{Glasses: testGlasses()}, dataStore: ds}
	shaken := []any{map[string]any{"kind": "ice"}, map[string]any{"kind": "mix"}}
	_, err = b.handleUpdateRecipes(map[string]any{"recipes": []any{
		map[string]any{"id": "shaken", "name": "Shaken", "glass": "martini", "steps": shaken},
	}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `recipe "shaken" needs a double_glass`)

	built := []any{map[string]any{"kind": "pour", "ingredient": "vodka", "oz": 2}}
	_, err = b.handleUpdateRecipes(map[string]any{"recipes": []any{
		map[string]any{"id": "built", "name": "Built", "glass": "martini", "steps": built},
	}})
	test.That(t, err, test.ShouldBeNil)
}

func TestGlassNeedOzMatchesTheWebApp(t *testing.T) {
	pours := []RecipeStep{{Kind: "pour", Oz: 2}, {Kind: "pour", Oz: 1}}
	steps := func(kinds ...string) []RecipeStep {
		out := append([]RecipeStep{}, pours...)
		for _, k := range kinds {
			out = append(out, RecipeStep{Kind: k})
		}
		return out
	}
	test.That(t, glassNeedOz(pours), test.ShouldEqual, 3.0)
	test.That(t, glassNeedOz(steps("ice", "mix", "strain")), test.ShouldEqual, 3*(1+meltFraction))
	test.That(t, glassNeedOz(steps("ice", "mix")), test.ShouldEqual, 3*(1+meltFraction)+iceInGlassOz)
}

func TestMakeCocktailRefusesDrinkTooBigForItsGlass(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	// Served unstrained, so the shaker's ice alone is more than the 2 oz glass holds.
	r := Recipe{ID: "iced", Name: "Iced", Steps: []RecipeStep{{Kind: "ice"}, {Kind: "mix"}}, Glass: "shot", DoubleGlass: "highball"}
	test.That(t, ds.UpdateRecipes([]Recipe{r}), test.ShouldBeNil)
	glasses := append(testGlasses(), Glass{ID: "shot", Name: "Shot", CapacityOz: 2, PoseSwitcher: "serve-shot"})
	b := &bartender{cfg: &Config{RecipeDefaults: &RecipeDefaults{}, Glasses: glasses}, dataStore: ds, queue: order.NewQueue()}

	_, err = b.handleMakeCocktail(context.Background(), map[string]any{"drink_id": "iced"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "the Shot glass holds 2 oz")
	test.That(t, len(b.queue.State().Queue), test.ShouldEqual, 0)

	_, err = b.handleMakeCocktail(context.Background(), map[string]any{"drink_id": "iced", "double": true})
	test.That(t, err, test.ShouldBeNil)
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

func stationTestBartender(t *testing.T, station string) *bartender {
	return &bartender{
		logger:          logging.NewTestLogger(t),
		cfg:             &Config{ArmName: "arm", GripperName: "gripper", Glasses: testGlasses(), DefaultGlassObstacle: "glass-center"},
		stationObstacle: station,
	}
}

func TestGlassObstaclesAreDependencies(t *testing.T) {
	cfg := Config{
		ArmName:              "arm",
		GripperName:          "gripper",
		PoseSwitcherNames:    []string{"serve-martini", "serve-highball"},
		Glasses:              testGlasses(),
		DefaultGlassObstacle: "glass-center",
	}
	deps, _, err := cfg.Validate("bartender")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldContain, gripper.Named("martini-glass").String())
	test.That(t, deps, test.ShouldContain, gripper.Named("glass-center").String())
}

func TestStationObstacleForGlass(t *testing.T) {
	cfg := &Config{DefaultGlassObstacle: "glass-center"}
	glasses := testGlasses()
	test.That(t, cfg.stationObstacleFor(&glasses[0]), test.ShouldEqual, "martini-glass")
	test.That(t, cfg.stationObstacleFor(&glasses[1]), test.ShouldEqual, "glass-center")
	test.That(t, cfg.stationObstacleFor(nil), test.ShouldEqual, "glass-center")
}

func TestGlassServe(t *testing.T) {
	b := stationTestBartender(t, "glass-center")
	glasses := testGlasses()

	sw, allow := b.glassServe(&glasses[0], "serving-glass-center")
	test.That(t, sw, test.ShouldEqual, "serve-martini")
	test.That(t, allow, test.ShouldResemble, b.heldVsObstacleAllowed("martini-glass"))

	sw, allow = b.glassServe(&glasses[1], "serving-glass-center")
	test.That(t, sw, test.ShouldEqual, "serve-highball")
	test.That(t, allow, test.ShouldResemble, b.heldVsObstacleAllowed("glass-center"))

	sw, allow = b.glassServe(nil, "serving-glass-center")
	test.That(t, sw, test.ShouldEqual, "serving-glass-center")
	test.That(t, allow, test.ShouldBeNil)
}

func TestWithAllowedCollisionsLeavesInputUnchanged(t *testing.T) {
	b := stationTestBartender(t, "martini-glass")
	overGlass := b.withAllowedCollisions(nil, b.heldVsObstacleAllowed("martini-glass"))
	before := len(overGlass.CollisionSpecification)

	tilt := b.withAllowedCollisions(overGlass, []AllowedCollision{{Frame1: "claws-middle", Frame2: "table"}})
	test.That(t, len(tilt.CollisionSpecification), test.ShouldEqual, before+1)
	test.That(t, len(overGlass.CollisionSpecification), test.ShouldEqual, before)

	approach := b.withAllowedCollisions(overGlass, []AllowedCollision{{Frame1: "claws-middle", Frame2: "shelf"}})
	test.That(t, approach.CollisionSpecification[before].Allows[0].Frame2, test.ShouldEqual, "shelf")
	test.That(t, tilt.CollisionSpecification[before].Allows[0].Frame2, test.ShouldEqual, "table")
}

func TestPourIntoGlassUsesGlassPosesAndChecksThemBeforePickup(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateSlotAssignment("vodka", "vodka"), test.ShouldBeNil)
	test.That(t, ds.UpdatePourerOz("vodka", 1), test.ShouldBeNil)

	asked := map[string][]any{}
	poseSwitch := func(name string, missing string) *inject.Switch {
		sw := inject.NewSwitch(name)
		sw.DoFunc = func(_ context.Context, cmd map[string]any) (map[string]any, error) {
			asked[name] = append(asked[name], cmd["get_pose_by_name"])
			if cmd["get_pose_by_name"] == missing {
				return nil, errors.New("no such pose")
			}
			return map[string]any{}, nil
		}
		return sw
	}
	b := stationTestBartender(t, "glass-center")
	b.dataStore = ds
	b.poseSwitches = []toggleswitch.Switch{
		poseSwitch("vodka", ""),
		poseSwitch("stations", ""),
		poseSwitch("serve-highball", posePourGlassRecover),
	}
	glasses := testGlasses()

	err = b.pourIntoGlass(context.Background(), "vodka", 1, &glasses[1])
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such pose")
	test.That(t, asked["serve-highball"], test.ShouldResemble, []any{posePourGlassApproach, posePourGlassTilt, posePourGlassRecover})
	test.That(t, asked["stations"], test.ShouldBeNil)
	test.That(t, asked["vodka"], test.ShouldBeNil)
}

func TestPrepareDrinkPlacesOrderGlassAtStation(t *testing.T) {
	b := stationTestBartender(t, "glass-center")
	var during string
	b.brew = func(context.Context, drinkPlan) error {
		during = b.stationObstacle
		return nil
	}
	glasses := testGlasses()
	payload, err := newCocktailJob(drinkPlan{title: "Iced", glass: &glasses[0]}, 1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b.prepareDrink(context.Background(), order.Order{ID: "1", Payload: payload}), test.ShouldBeNil)
	test.That(t, during, test.ShouldEqual, "martini-glass")
	test.That(t, b.stationObstacle, test.ShouldEqual, "martini-glass")

	payload, err = newCocktailJob(drinkPlan{title: "Plain"}, 1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b.prepareDrink(context.Background(), order.Order{ID: "2", Payload: payload}), test.ShouldBeNil)
	test.That(t, during, test.ShouldEqual, "glass-center")
}

func TestPlanningWorldHasOnlyStationGlass(t *testing.T) {
	b := stationTestBartender(t, "martini-glass")
	fsSvc := inject.NewFrameSystemService("fs")
	fsSvc.FrameSystemConfigFunc = func(context.Context) (*framesystem.Config, error) {
		var parts []*referenceframe.FrameSystemPart
		for _, name := range []string{"martini-glass", "highball-glass", "glass-center", "table"} {
			box, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 10, Y: 10, Z: 10}, name)
			if err != nil {
				return nil, err
			}
			lif := referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), name, box)
			parts = append(parts, &referenceframe.FrameSystemPart{FrameConfig: lif})
		}
		return &framesystem.Config{Parts: parts}, nil
	}
	a := inject.NewArm("arm")
	a.CurrentInputsFunc = func(context.Context) ([]referenceframe.Input, error) { return nil, nil }
	b.fsSvc, b.arm = fsSvc, a
	b.cfg.Glasses[1].Obstacle = "highball-glass"

	fs, _, err := b.currentInputs(context.Background())
	test.That(t, err, test.ShouldBeNil)
	names := fs.FrameNames()
	test.That(t, names, test.ShouldContain, "martini-glass")
	test.That(t, names, test.ShouldContain, "table")
	test.That(t, names, test.ShouldNotContain, "highball-glass")
	test.That(t, names, test.ShouldNotContain, "glass-center")
}

func TestAllowsForAbsentGlassesAreDropped(t *testing.T) {
	b := stationTestBartender(t, "martini-glass")
	c := b.withAllowedCollisions(nil, []AllowedCollision{
		{Frame1: "claws-middle", Frame2: "glass-center"},
		{Frame1: "claws-middle", Frame2: "martini-glass"},
		{Frame1: "claws-middle", Frame2: "table"},
	})
	var kept []string
	for _, spec := range c.CollisionSpecification {
		for _, a := range spec.Allows {
			kept = append(kept, a.Frame2)
		}
	}
	test.That(t, kept, test.ShouldResemble, []string{"martini-glass", "table"})
}
