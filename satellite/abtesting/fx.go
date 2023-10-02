//go:build !noabtest

package abtesting

import (
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net/http"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/modules"
)

func CreateService(log *zap.Logger, config Config) *Service {
	return NewService(log.Named("abtesting:service"), config)
}

func CreateABTesting() lifecycle.Item {
	return lifecycle.Item{
		Name: "abtesting:service",
	}
}

func RegisterConsoleEndpoint(logger *zap.Logger, config Config, service *Service) consoleweb.ConsoleExtension {
	return func(router *mux.Router, ch mux.MiddlewareFunc, ah mux.MiddlewareFunc) {
		//if config.Enabled {
		abController := NewABTesting(logger, service)
		abRouter := router.PathPrefix("/api/v0/ab").Subrouter()
		abRouter.Use(ch)
		abRouter.Use(ah)
		abRouter.Handle("/values", http.HandlerFunc(abController.GetABValues)).Methods(http.MethodGet, http.MethodOptions)
		abRouter.Handle("/hit/{action}", http.HandlerFunc(abController.SendHit)).Methods(http.MethodPost, http.MethodOptions)
		//}
	}
}

var Module = fx.Module("abtest",
	fx.Provide(
		CreateService,
		fx.Annotate(CreateABTesting, fx.ResultTags(`group:"service"`)),
		fx.Annotate(RegisterConsoleEndpoint, fx.ResultTags(`group:"console"`)),
	),
)

func init() {
	modules.AutoRegistered = append(modules.AutoRegistered, Module)
	registered := false
	modules.DynamicConfigs = append(modules.DynamicConfigs, func(cmd *cobra.Command, opts ...cfgstruct.BindOpt) {
		cfg := Config{}
		if cmd.Use == "api" {
			opts = append(opts, cfgstruct.Prefix("console.ab-testing."))
			process.Bind(cmd, &cfg, opts...)
		}
		if !registered {
			modules.AutoRegistered = append(modules.AutoRegistered, fx.Supply(cfg))
			registered = true
		}
	})
}
