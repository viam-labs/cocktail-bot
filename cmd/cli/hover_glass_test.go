package main

import (
	"testing"

	"go.viam.com/test"
)

func TestHoverGlassFlagsValidate(t *testing.T) {
	base := HoverGlassFlags{MachineAddress: "m", Component: "gripper"}

	local := base
	local.Camera = "cam"
	test.That(t, local.validate(), test.ShouldBeNil)

	spill := base
	spill.GlassFinder = "spill-glass-finder"
	test.That(t, spill.validate(), test.ShouldBeNil)

	noSource := base
	test.That(t, noSource.validate(), test.ShouldBeError, "hover-glass: --camera is required unless --glass-finder is set")

	spillPCD := spill
	spillPCD.SavePCD = "glass.pcd"
	test.That(t, spillPCD.validate(), test.ShouldBeError, "hover-glass: --save-pcd is not supported with --glass-finder")
}
