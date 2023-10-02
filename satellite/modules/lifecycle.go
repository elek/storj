package modules

import (
	"go.uber.org/fx"
	"storj.io/storj/private/lifecycle"
)

var AutoRegistered []fx.Option

type RegisteredServices struct {
	Services []lifecycle.Item
}

var Modules = fx.Module("services",
	fx.Provide(
		fx.Annotate(
			func(item ...lifecycle.Item) RegisteredServices {
				return RegisteredServices{
					Services: item,
				}
			},
			fx.ParamTags(`group:"service"`)),
	),
)
