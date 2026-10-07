package main

import (
	"testing"

	"go.viam.com/test"
)

func TestPourGlassFlagsValidate(t *testing.T) {
	findOnly := PourGlassFlags{MachineAddress: "m", FindOnly: true}
	test.That(t, findOnly.validate(), test.ShouldBeNil)

	missing := PourGlassFlags{MachineAddress: "m"}
	test.That(t, missing.validate(), test.ShouldNotBeNil)

	full := PourGlassFlags{MachineAddress: "m", Bottle: "bottle-gin", PourMs: 1000}
	test.That(t, full.validate(), test.ShouldBeNil)

	noAddr := full
	noAddr.MachineAddress = ""
	test.That(t, noAddr.validate(), test.ShouldNotBeNil)
}

func TestPourGlassCommand(t *testing.T) {
	findOnly := PourGlassFlags{FindOnly: true}
	_, ok := findOnly.command()["find_glass"]
	test.That(t, ok, test.ShouldBeTrue)

	full := PourGlassFlags{Bottle: "bottle-gin", PourMs: 1500, MouthOffsetMM: 100}
	test.That(t, full.command(), test.ShouldResemble, map[string]interface{}{
		"find_and_pour": map[string]interface{}{"bottle": "bottle-gin", "pour_ms": 1500},
	})

	full.MouthOffsetMM = 80
	full.MouthOffsetSet = true
	test.That(t, full.command(), test.ShouldResemble, map[string]interface{}{
		"find_and_pour": map[string]interface{}{"bottle": "bottle-gin", "pour_ms": 1500, "mouth_offset_mm": 80.0},
	})
}
