// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/modules"
	"storj.io/storj/satellite/overlay"

	"storj.io/private/process"
	"storj.io/storj/satellite"
)

func cmdRangedLoopRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	errCh := make(chan error)

	app := fx.New(modules.Modules,
		fx.WithLogger(func(logger *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: logger}
		}),
		fx.Module("autoregistered", modules.AutoRegistered...),
		fx.Invoke(func(log *zap.Logger) {
			if err := process.InitMetricsWithHostname(ctx, log, nil); err != nil {
				log.Warn("Failed to initialize telemetry batcher on satellite api", zap.Error(err))
			}
		}),
		fx.Invoke(func(loop *rangedloop.Service) {}),
		satellite.RangedLoopModule,
		fx.Provide(
			createLogger,
			createRevocationDB(ctx),
			loadIdentity,
			nodeURL,
			createAccountingCache(ctx),
			func() *satellite.Config {
				return &runCfg.Config
			},
			func() *zap.AtomicLevel {
				return process.AtomicLevel(cmd)
			},
			func(config *satellite.Config) (overlay.PlacementRules, error) {
				pd, err := config.Placement.Parse()
				if err != nil {
					return nil, err
				}
				return pd.CreateFilters, nil
			},
			exposeConfig,
			exposeDB,
			fx.Annotate(OpenSatelliteDB, fx.OnStart(MigrateSatelliteDB), fx.OnStop(CloseSatelliteDB)),
			fx.Annotate(OpeMetabaseDB, fx.OnStart(MigrateMetabaseDB), fx.OnStop(CloseMetabaseDB)),
		),
		overlay.Module,
	)

	err = app.Start(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	select {
	case <-app.Wait():
	case err = <-errCh:
	}

	return errs.Combine(err, app.Stop(ctx))
}
