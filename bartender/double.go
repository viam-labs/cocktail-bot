package bartender

import "fmt"

// Everything a double changes about a drink is decided in planDrink.
type drinkPlan struct {
	title      string
	steps      []RecipeStep
	servePhase string
	double     bool
	// nil serves at recipe_defaults.serve_station with the shared direct-pour poses.
	glass *Glass
}

func usesShaker(steps []RecipeStep) bool {
	for _, s := range steps {
		switch s.Kind {
		case "ice", "mix", "strain":
			return true
		}
	}
	return false
}

// Drinks that skip the shaker pour straight into the serving glass, which a double would overflow.
// A drink with a glass but no double_glass has nowhere to serve the double.
func canDouble(r Recipe) bool {
	if r.Glass != "" && r.DoubleGlass == "" {
		return false
	}
	return usesShaker(stepsFor(r))
}

func planDrink(name string, r Recipe, double bool) (drinkPlan, error) {
	steps := stepsFor(r)
	if !double {
		return drinkPlan{title: name, steps: steps, servePhase: "Serving"}, nil
	}
	if !usesShaker(steps) {
		return drinkPlan{}, fmt.Errorf("%q can't be doubled: only drinks made in the shaker can be doubled", name)
	}
	doubled := make([]RecipeStep, len(steps))
	for i, s := range steps {
		if s.Kind == "pour" {
			s.Oz *= 2
		}
		doubled[i] = s
	}
	return drinkPlan{
		title:      "2 × " + name,
		steps:      doubled,
		servePhase: "Serving into a large glass",
		double:     true,
	}, nil
}
