// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"storj.io/common/identity"
	"storj.io/common/peertls/extensions"
	"storj.io/common/storj"
	"storj.io/private/version"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/modules"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/satellite/repair/queue"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/private/process"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite"
	_ "storj.io/storj/satellite/abtesting"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/metabase"
	_ "storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb"
)

func loadIdentity(log *zap.Logger) (*identity.FullIdentity, error) {
	fullIdentity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return nil, errs.New("Failed to load identity: %+v", err)
	}
	return fullIdentity, nil
}

func nodeURL(config *satellite.Config, identity *identity.FullIdentity) storj.NodeURL {
	return storj.NodeURL{
		ID:      identity.ID,
		Address: config.Contact.ExternalAddress,
	}
}

func createRevocationDB(ctx context.Context) func(lifecycle fx.Lifecycle) (extensions.RevocationDB, error) {
	return func(lifecycle fx.Lifecycle) (extensions.RevocationDB, error) {
		revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.Config.Server.Config)
		if err != nil {
			return nil, errs.New("Error creating revocation database on satellite api: %+v", err)
		}
		lifecycle.Append(fx.StopHook(func() error {
			return errs.Combine(err, revocationDB.Close())
		}))
		return revocationDB, nil
	}
}

func createAccountingCache(ctx context.Context) func(log *zap.Logger, lifecycle fx.Lifecycle) (accounting.Cache, error) {
	return func(log *zap.Logger, lifecycle fx.Lifecycle) (accounting.Cache, error) {
		accountingCache, err := live.OpenCache(ctx, log.Named("live-accounting"), runCfg.LiveAccounting)
		if err != nil {
			if !accounting.ErrSystemOrNetError.Has(err) || accountingCache == nil {
				return nil, errs.New("Error instantiating live accounting cache: %w", err)
			}

			log.Warn("Unable to connect to live accounting cache. Verify connection",
				zap.Error(err),
			)
		}
		lifecycle.Append(fx.StopHook(func() error {
			return accountingCache.Close()
		}))
		return accountingCache, nil
	}
}

func newRollupsWriteCache(log *zap.Logger, db satellite.DB) *orders.RollupsWriteCache {
	return orders.NewRollupsWriteCache(log.Named("orders-write-cache"), db.Orders(), runCfg.Orders.FlushBatchSize)
}

func closeRollupWriteCache(ctx context.Context, rollupsWriteCache *orders.RollupsWriteCache) error {
	return rollupsWriteCache.CloseAndFlush(context2.WithoutCancellation(ctx))
}

func createLogger() *zap.Logger {
	return zap.L()

}

type AllConfig struct {
	fx.Out
	Console    consoleweb.Config
	Rangedloop rangedloop.Config
	Audit      audit.Config
	Graceful   gracefulexit.Config
	Checker    checker.Config
	Overlay    overlay.Config
}

func exposeConfig(cfg *satellite.Config) AllConfig {
	return AllConfig{
		Console:    cfg.Console,
		Rangedloop: cfg.RangedLoop,
		Audit:      cfg.Audit,
		Graceful:   cfg.GracefulExit,
		Checker:    cfg.Checker,
		Overlay:    cfg.Overlay,
	}
}

type AllDB struct {
	fx.Out
	OIDC                  oidc.DB
	VerifyQueue           audit.VerifyQueue
	Overlay               overlay.DB
	GracefulExit          gracefulexit.DB
	StoragenodeAccounting accounting.StoragenodeAccounting
	RepairQueue           queue.RepairQueue
	NodeEvents            nodeevents.DB
}

func exposeDB(db satellite.DB) AllDB {
	return AllDB{
		OIDC:                  db.OIDC(),
		VerifyQueue:           db.VerifyQueue(),
		GracefulExit:          db.GracefulExit(),
		Overlay:               db.OverlayCache(),
		StoragenodeAccounting: db.StoragenodeAccounting(),
		RepairQueue:           db.RepairQueue(),
		NodeEvents:            db.NodeEvents(),
	}
}

func cmdAPIRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	errCh := make(chan error)

	app := fx.New(modules.Modules,
		fx.WithLogger(func(logger *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: logger}
		}),
		fx.Module("autoregistered", modules.AutoRegistered...),
		consoleweb.Module,
		fx.Invoke(func(log *zap.Logger) {
			if err := process.InitMetricsWithHostname(ctx, log, nil); err != nil {
				log.Warn("Failed to initialize telemetry batcher on satellite api", zap.Error(err))
			}
		}),
		fx.Invoke(func(*satellite.API) {}),
		fx.Provide(
			createLogger,
			createRevocationDB(ctx),
			loadIdentity,
			nodeURL,
			createAccountingCache(ctx),
			func() *satellite.Config {
				return &runCfg.Config
			},
			exposeConfig,
			exposeDB,
			func() version.Info {
				return version.Info{}
			},

			//fx.Invoke(func() {
			//	//_, err = peer.Version.Service.CheckVersion(ctx)
			//	//if err != nil {
			//	//	return err
			//	//}
			//
			//})
			fx.Annotate(OpenSatelliteDB, fx.OnStart(MigrateSatelliteDB), fx.OnStop(CloseSatelliteDB)),
			fx.Annotate(OpeMetabaseDB, fx.OnStart(MigrateMetabaseDB), fx.OnStop(CloseMetabaseDB)),
			fx.Annotate(newRollupsWriteCache, fx.OnStop(closeRollupWriteCache)),
			fx.Annotate(
				satellite.NewAPI,
				fx.OnStart(func(ctx context.Context, peer *satellite.API) error {
					go func() {
						errCh <- peer.Run(ctx)
					}()
					return nil
				}),
				fx.OnStop(func(peer *satellite.API) error {
					return peer.Close()
				}),
			),
		),
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

func MigrateSatelliteDB(ctx context.Context, log *zap.Logger, db satellite.DB) error {
	var err error
	for _, migration := range strings.Split(runCfg.DatabaseOptions.MigrationUnsafe, ",") {
		switch migration {
		case fullMigration:
			err = db.MigrateToLatest(ctx)
			if err != nil {
				return err
			}
		case snapshotMigration:
			log.Info("MigrationUnsafe using latest snapshot. It's not for production", zap.String("db", "master"))
			err = db.Testing().TestMigrateToLatest(ctx)
			if err != nil {
				return err
			}
		case testDataCreation:
			err := createTestData(ctx, db)
			if err != nil {
				return err
			}
		case noMigration:
		// noop
		default:
			return errs.New("unsupported migration type: %s, please try one of the: %s", migration, strings.Join(migrationTypes, ","))
		}
	}

	err = db.CheckVersion(ctx)
	if err != nil {
		log.Error("Failed satellite database version check.", zap.Error(err))
		return errs.New("Error checking version for satellitedb: %+v", err)
	}
	return nil
}

func OpenSatelliteDB(log *zap.Logger) (satellite.DB, error) {
	// TODO
	ctx := context.Background()
	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName:      "satellite-api",
		APIKeysLRUOptions:    runCfg.APIKeysLRUOptions(),
		RevocationLRUOptions: runCfg.RevocationLRUOptions(),
	})
	if err != nil {
		return db, errs.New("Error starting master database on satellite api: %+v", err)
	}
	return db, nil
}

func CloseSatelliteDB(db satellite.DB) error {
	return db.Close()
}

func OpeMetabaseDB(log *zap.Logger) (*metabase.DB, error) {
	//TODO
	ctx := context.TODO()
	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Config.Metainfo.DatabaseURL,
		runCfg.Config.Metainfo.Metabase("satellite-api"))
	if err != nil {
		return nil, errs.New("Error creating metabase connection on satellite api: %+v", err)
	}
	return metabaseDB, nil
}

func CloseMetabaseDB(db *metabase.DB) error {
	return db.Close()
}

func MigrateMetabaseDB(ctx context.Context, log *zap.Logger, metabaseDB *metabase.DB) (err error) {
	for _, migration := range strings.Split(runCfg.DatabaseOptions.MigrationUnsafe, ",") {
		switch migration {
		case fullMigration:
			err = metabaseDB.MigrateToLatest(ctx)
			if err != nil {
				return err
			}
		case snapshotMigration:
			log.Info("MigrationUnsafe using latest snapshot. It's not for production", zap.String("db", "master"))
			err = metabaseDB.TestMigrateToLatest(ctx)
			if err != nil {
				return err
			}
		case noMigration, testDataCreation:
		// noop
		default:
			return errs.New("unsupported migration type: %s, please try one of the: %s", migration, strings.Join(migrationTypes, ","))
		}
	}

	err = metabaseDB.CheckVersion(ctx)
	if err != nil {
		log.Error("Failed metabase database version check.", zap.Error(err))
		return errs.New("failed metabase version check: %+v", err)
	}
	return nil
}
