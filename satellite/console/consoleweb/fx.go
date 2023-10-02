package consoleweb

import (
	"go.uber.org/fx"
	"storj.io/storj/private/lifecycle"
)

type RegisteredExtensions struct {
	Extensions []ConsoleExtension
}

func NewConsole(server *Server) lifecycle.Item {
	return lifecycle.Item{
		Name:  "console:endpoint",
		Run:   server.Run,
		Close: server.Close,
	}
}

var Module = fx.Module("console",
	fx.Provide(
		//NewServer,
		//fx.Annotate(NewConsole, fx.ResultTags(`group:"service"`)),
		fx.Annotate(
			func(items ...ConsoleExtension) RegisteredExtensions {
				return RegisteredExtensions{
					Extensions: items,
				}
			},
			fx.ParamTags(`group:"console"`)),
	),
)
