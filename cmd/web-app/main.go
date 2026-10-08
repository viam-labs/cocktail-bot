package main

import (
	"context"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var model = resource.NewModel("viam", "cocktail-bot-app", "cocktail-bot")

type placeholder struct {
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	name resource.Name
}

func (p *placeholder) Name() resource.Name { return p.name }

func (p *placeholder) Status(_ context.Context) (map[string]any, error) {
	return map[string]any{}, nil
}

func (p *placeholder) DoCommand(_ context.Context, _ map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}

func init() {
	resource.RegisterComponent(generic.API, model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				_ logging.Logger,
			) (resource.Resource, error) {
				return &placeholder{name: conf.ResourceName()}, nil
			},
		},
	)
}

func main() {
	module.ModularMain(resource.APIModel{API: generic.API, Model: model})
}
