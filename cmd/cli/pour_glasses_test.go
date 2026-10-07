package main

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestPickGlassCentersDropsDuplicates(t *testing.T) {
	centers := []r3.Vector{
		{X: 500, Y: 0, Z: 80},
		{X: 520, Y: 10, Z: 120},
		{X: 650, Y: 100, Z: 80},
	}
	got := pickGlassCenters(centers, 50, 0)
	test.That(t, got, test.ShouldResemble, []r3.Vector{centers[0], centers[2]})
}

func TestPickGlassCentersIgnoresZForSeparation(t *testing.T) {
	got := pickGlassCenters([]r3.Vector{{X: 0, Y: 0, Z: 0}, {X: 10, Y: 0, Z: 500}}, 50, 0)
	test.That(t, len(got), test.ShouldEqual, 1)
}

func TestPickGlassCentersMax(t *testing.T) {
	centers := []r3.Vector{{X: 0}, {X: 100}, {X: 200}}
	test.That(t, pickGlassCenters(centers, 50, 2), test.ShouldResemble, centers[:2])
	test.That(t, pickGlassCenters(nil, 50, 0), test.ShouldBeEmpty)
}

func TestPourGlassesFlagsValidate(t *testing.T) {
	base := PourGlassesFlags{MachineAddress: "m", Camera: "cam"}
	dry := base
	dry.DryRun = true
	test.That(t, dry.validate(), test.ShouldBeNil)

	test.That(t, base.validate(), test.ShouldNotBeNil)
	full := base
	full.Bottle = "bottle-gin"
	full.PourMs = 1000
	test.That(t, full.validate(), test.ShouldBeNil)

	noCam := full
	noCam.Camera = ""
	test.That(t, noCam.validate(), test.ShouldNotBeNil)
}
