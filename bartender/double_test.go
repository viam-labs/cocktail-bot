package bartender

import (
	"context"
	"testing"

	"go.viam.com/test"
)

func shakenRecipe() Recipe {
	return Recipe{
		ID:   "margarita",
		Name: "Margarita",
		Pours: []Pour{
			{Ingredient: "tequila", Oz: 1.5},
			{Ingredient: "triple-sec", Oz: 1},
		},
	}
}

func builtRecipe() Recipe {
	return Recipe{
		ID:    "neat",
		Name:  "Neat",
		Steps: []RecipeStep{{Kind: "pour", Ingredient: "whiskey", Oz: 2}},
	}
}

func TestPlanDrinkSingle(t *testing.T) {
	plan, err := planDrink("Margarita", shakenRecipe(), false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan.title, test.ShouldEqual, "Margarita")
	test.That(t, plan.servePhase, test.ShouldEqual, "Serving")
	test.That(t, plan.double, test.ShouldBeFalse)
	test.That(t, plan.steps, test.ShouldResemble, stepsFor(shakenRecipe()))
}

func TestPlanDrinkDoubleDoublesPoursOnly(t *testing.T) {
	r := shakenRecipe()
	plan, err := planDrink("Margarita", r, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan.title, test.ShouldEqual, "2 × Margarita")
	test.That(t, plan.servePhase, test.ShouldEqual, "Serving into a large glass")
	test.That(t, plan.double, test.ShouldBeTrue)
	test.That(t, plan.steps, test.ShouldResemble, []RecipeStep{
		{Kind: "ice", IceType: "crushed"},
		{Kind: "pour", Ingredient: "tequila", Oz: 3},
		{Kind: "pour", Ingredient: "triple-sec", Oz: 2},
		{Kind: "mix", MixMode: "shake"},
		{Kind: "strain"},
	})
	test.That(t, r.Pours[0].Oz, test.ShouldEqual, 1.5)
}

func TestPlanDrinkDoubleLeavesExplicitStepsUntouched(t *testing.T) {
	r := Recipe{Name: "Stirred", Steps: []RecipeStep{
		{Kind: "pour", Ingredient: "gin", Oz: 2},
		{Kind: "mix", MixMode: "stir"},
	}}
	_, err := planDrink("Stirred", r, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Steps[0].Oz, test.ShouldEqual, 2.0)
}

func TestPlanDrinkRefusesDoubleWithoutShaker(t *testing.T) {
	_, err := planDrink("Neat", builtRecipe(), true)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "can't be doubled")
}

func TestCanDouble(t *testing.T) {
	test.That(t, canDouble(shakenRecipe()), test.ShouldBeTrue)
	test.That(t, canDouble(builtRecipe()), test.ShouldBeFalse)
	off := false
	pourOnly := Recipe{Pours: []Pour{{Ingredient: "vodka", Oz: 2}}, CrushedIce: &off, Shake: &off, Strain: &off}
	test.That(t, canDouble(pourOnly), test.ShouldBeFalse)
}

func TestParseMakeCocktailDouble(t *testing.T) {
	_, _, _, double, err := parseMakeCocktail(map[string]any{"drink_id": "margarita"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, double, test.ShouldBeFalse)

	_, _, _, double, err = parseMakeCocktail(map[string]any{"drink_id": "margarita", "double": true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, double, test.ShouldBeTrue)

	_, _, _, _, err = parseMakeCocktail(map[string]any{"drink_id": "margarita", "double": "yes"})
	test.That(t, err, test.ShouldNotBeNil)

	_, _, _, _, err = parseMakeCocktail(map[string]any{"drink_id": "margarita", "double": true, "quantity": 2.0})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "quantity")
}

func TestMakeCocktailRefusesDoubleWithoutShaker(t *testing.T) {
	b := &bartender{cfg: &Config{RecipeDefaults: &RecipeDefaults{}}}
	_, err := b.handleMakeCocktail(context.Background(), map[string]any{
		"recipe": map[string]any{
			"name":  "Neat",
			"steps": []any{map[string]any{"kind": "pour", "ingredient": "whiskey", "oz": 2.0}},
		},
		"double": true,
	})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "can't be doubled")
	test.That(t, b.status.snapshot().busy, test.ShouldBeFalse)
}

func TestGetRecipesExposesCanDouble(t *testing.T) {
	ds, err := newDataStore(t.TempDir())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ds.UpdateRecipes([]Recipe{shakenRecipe(), builtRecipe()}), test.ShouldBeNil)
	b := &bartender{dataStore: ds}

	resp, err := b.handleGetRecipes()
	test.That(t, err, test.ShouldBeNil)
	recipes := resp["recipes"].([]any)
	test.That(t, len(recipes), test.ShouldEqual, 2)
	byID := map[string]map[string]any{}
	for _, r := range recipes {
		m := r.(map[string]any)
		byID[m["id"].(string)] = m
	}
	test.That(t, byID["margarita"]["can_double"], test.ShouldEqual, true)
	test.That(t, byID["margarita"]["name"], test.ShouldEqual, "Margarita")
	test.That(t, byID["neat"]["can_double"], test.ShouldEqual, false)
}

func TestAlertMarksDouble(t *testing.T) {
	a := orderAlert{drink: "2 × Margarita", double: true, orderID: "o1"}
	test.That(t, alertText(a), test.ShouldContainSubstring, "2 × Margarita (double)")
	blocks := alertBlocks(a)
	fields := blocks[1].(map[string]any)["fields"].([]any)
	test.That(t, fields[1].(map[string]any)["text"], test.ShouldEqual, "*Size:*\nDouble, one large glass")

	single := alertText(orderAlert{drink: "Margarita"})
	test.That(t, single, test.ShouldNotContainSubstring, "double")
}
