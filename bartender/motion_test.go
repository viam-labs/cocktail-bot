package bartender

import (
	"math"
	"testing"

	"go.viam.com/test"
)

func TestMoveOptionsFromCfgNilWhenZero(t *testing.T) {
	test.That(t, moveOptionsFromCfg(0, 0), test.ShouldBeNil)
}

func TestMoveOptionsFromCfgConvertsDegsToRads(t *testing.T) {
	opts := moveOptionsFromCfg(180, 90)
	test.That(t, opts, test.ShouldNotBeNil)
	test.That(t, opts.MaxVelRads, test.ShouldAlmostEqual, math.Pi, 1e-9)
	test.That(t, opts.MaxAccRads, test.ShouldAlmostEqual, math.Pi/2, 1e-9)
}

func TestMoveOptionsFromCfgVelOnly(t *testing.T) {
	opts := moveOptionsFromCfg(60, 0)
	test.That(t, opts, test.ShouldNotBeNil)
	test.That(t, opts.MaxVelRads, test.ShouldAlmostEqual, math.Pi/3, 1e-9)
	test.That(t, opts.MaxAccRads, test.ShouldEqual, 0.0)
}

func TestMoveOptionsFromCfgAccOnly(t *testing.T) {
	opts := moveOptionsFromCfg(0, 60)
	test.That(t, opts, test.ShouldNotBeNil)
	test.That(t, opts.MaxVelRads, test.ShouldEqual, 0.0)
	test.That(t, opts.MaxAccRads, test.ShouldAlmostEqual, math.Pi/3, 1e-9)
}
