package bartender

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Glass is a type of glass staff can place at the serving station, defined in the bartender config.
// PoseSwitcher holds the serve poses taught for this glass; Obstacle models the glass in the world.
type Glass struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	CapacityOz   float64 `json:"capacity_oz"`
	Image        string  `json:"image,omitempty"`
	PoseSwitcher string  `json:"pose_switcher"`
	Obstacle     string  `json:"obstacle,omitempty"`
}

// glassImages are the outlines the web app can draw.
var glassImages = map[string]bool{"highball": true, "martini": true, "rocks": true, "coupe": true, "wine": true}

func validateGlasses(path string, glasses []Glass, switchers map[string]struct{}) error {
	seen := map[string]bool{}
	var problems []string
	for i, g := range glasses {
		at := fmt.Sprintf("%s.glasses[%d]", path, i)
		switch {
		case g.ID == "":
			problems = append(problems, at+": id is required")
		case seen[g.ID]:
			problems = append(problems, fmt.Sprintf("%s: id %q is used twice", at, g.ID))
		}
		seen[g.ID] = true
		if strings.TrimSpace(g.Name) == "" {
			problems = append(problems, at+": name is required")
		}
		if g.CapacityOz <= 0 {
			problems = append(problems, at+": capacity_oz must be > 0")
		}
		if g.Image != "" && !glassImages[g.Image] {
			problems = append(problems, fmt.Sprintf("%s: image %q must be one of highball, martini, rocks, coupe, wine", at, g.Image))
		}
		if g.PoseSwitcher == "" {
			problems = append(problems, at+": pose_switcher is required")
		} else if _, ok := switchers[g.PoseSwitcher]; !ok {
			problems = append(problems, fmt.Sprintf("%s: pose_switcher %q is not in pose_switcher_names", at, g.PoseSwitcher))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
}

func findGlass(glasses []Glass, id string) (Glass, bool) {
	for _, g := range glasses {
		if g.ID == id {
			return g, true
		}
	}
	return Glass{}, false
}

// unknownGlassRefs lists recipe fields that name a glass missing from the config.
func unknownGlassRefs(recipes []Recipe, glasses []Glass) []string {
	var out []string
	for _, r := range recipes {
		if _, ok := findGlass(glasses, r.Glass); r.Glass != "" && !ok {
			out = append(out, fmt.Sprintf("recipe %q uses glass %q", r.ID, r.Glass))
		}
		if _, ok := findGlass(glasses, r.DoubleGlass); r.DoubleGlass != "" && !ok {
			out = append(out, fmt.Sprintf("recipe %q uses double glass %q", r.ID, r.DoubleGlass))
		}
	}
	sort.Strings(out)
	return out
}

// servingGlass is the glass r is served in, or nil when the recipe names none.
func servingGlass(r Recipe, double bool, glasses []Glass) (*Glass, error) {
	id := r.Glass
	if double {
		id = r.DoubleGlass
	}
	if id == "" {
		return nil, nil
	}
	g, ok := findGlass(glasses, id)
	if !ok {
		return nil, fmt.Errorf("glass %q is not in the bartender's glasses config", id)
	}
	return &g, nil
}

// glassObstacleNames lists every obstacle that models a glass at the serving station, sorted.
// Only one of them is in the planning world at a time.
func (c *Config) glassObstacleNames() []string {
	seen := map[string]bool{}
	if c.DefaultGlassObstacle != "" {
		seen[c.DefaultGlassObstacle] = true
	}
	for _, g := range c.Glasses {
		if g.Obstacle != "" {
			seen[g.Obstacle] = true
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// stationObstacleFor is the obstacle at the station while a drink in glass is made.
// A glass with no obstacle leaves the station unmodeled.
func (c *Config) stationObstacleFor(glass *Glass) string {
	if glass != nil {
		return glass.Obstacle
	}
	if c == nil {
		return ""
	}
	return c.DefaultGlassObstacle
}

// absentGlassObstacles are the glass obstacles left out of planning because another glass is at the station.
func (b *bartender) absentGlassObstacles() map[string]bool {
	if b.cfg == nil {
		return nil
	}
	absent := map[string]bool{}
	for _, name := range b.cfg.glassObstacleNames() {
		if name != b.stationObstacle {
			absent[name] = true
		}
	}
	return absent
}
