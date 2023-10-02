package debug

import (
	"errors"
	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net"
	"storj.io/private/debug"
	"storj.io/private/process"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/satellite/modules"
)

func CreateDebug(config debug.Config, log *zap.Logger) lifecycle.Item { // setup debug
	var err error
	var listener net.Listener
	if config.Address != "" {
		listener, err = net.Listen("tcp", config.Address)
		if err != nil {
			withoutStack := errors.New(err.Error())
			log.Debug("failed to start debug endpoints", zap.Error(withoutStack))
		}
	}

	config.ControlTitle = "API"
	// TODO atomicLevel
	server := debug.NewServerWithAtomicLevel(log.Named("debug"), listener, monkit.Default, config, nil)
	return lifecycle.Item{
		Name:  "debug",
		Run:   server.Run,
		Close: server.Close,
	}
}

var Module = fx.Module("abtest",
	fx.Provide(
		fx.Annotate(CreateDebug, fx.ResultTags(`group:"service"`)),
		func() debug.Config {
			return debug.Config{
				Address: *process.DebugAddrFlag,
			}
		},
	),
)

func init() {
	modules.AutoRegistered = append(modules.AutoRegistered, Module)
}
