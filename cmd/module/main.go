package main

import (
	"github.com/viam-labs/cocktail-bot/bartender"

	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/generic"
)

func main() {
	module.ModularMain(
		resource.APIModel{API: generic.API, Model: bartender.Model},
	)
}
