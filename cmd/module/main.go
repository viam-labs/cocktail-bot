package main

import (
	"github.com/viam-labs/cocktail-bot/bartender"
	"github.com/viam-labs/cocktail-bot/glassfinder"
	"github.com/viam-labs/cocktail-bot/ordersensor"
	"github.com/viam-labs/cocktail-bot/poseswitcher"

	"go.viam.com/rdk/components/sensor"
	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/generic"
	"go.viam.com/rdk/services/vision"
)

func main() {
	module.ModularMain(
		resource.APIModel{API: generic.API, Model: bartender.Model},
		resource.APIModel{API: sensor.API, Model: ordersensor.Model},
		resource.APIModel{API: vision.API, Model: glassfinder.Model},
		resource.APIModel{API: toggleswitch.API, Model: poseswitcher.Model},
	)
}
