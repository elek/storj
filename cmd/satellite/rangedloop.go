// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"net/url"
	"storj.io/storj/private/mud"

	"storj.io/common/process"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
)

func cmdRangedLoopRun(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-rangedloop"})
	if err != nil {
		return errs.New("Error starting master database on satellite rangedloop: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	// this is a temporary workaround, as we don't support full spanner implementation (yet).
	// for now, we can use spanner overlay with using &spanner=.... db url fragment.
	// implemented methods will use the spanner implementation
	var adapters []metabase.Adapter
	parsedDbConnection, err := url.Parse(runCfg.Metainfo.DatabaseURL)
	if err != nil {
		return errs.Wrap(err)
	}
	if spannerConnection := parsedDbConnection.Query().Get("spanner"); spannerConnection != "" {
		log.Info("Initializing spanner connection", zap.String("connection", spannerConnection))
		ball := mud.NewBall()
		metabase.SpannerModule(ball, spannerConnection)
		for _, component := range mud.FindSelectedWithDependencies(ball, mud.Select[*metabase.SpannerAdapter](ball)) {
			err := component.Init(ctx)
			if err != nil {
				return errs.Wrap(err)
			}
		}

		adapter := mud.MustLookup[*metabase.SpannerAdapter](ball)
		if err != nil {
			return err
		}
		parsedDbConnection.Query().Del("spanner")
		runCfg.Metainfo.DatabaseURL = parsedDbConnection.String()
		adapters = append(adapters, adapter)
	}

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL, runCfg.Metainfo.Metabase("satellite-rangedloop"), adapters...)
	if err != nil {
		return errs.New("Error creating metabase connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()

	peer, err := satellite.NewRangedLoop(log, db, metabaseDB, &runCfg.Config, process.AtomicLevel(cmd))
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithHostname(ctx, log, nil); err != nil {
		log.Warn("Failed to initialize telemetry on satellite rangedloop", zap.Error(err))
	}

	if err := metabaseDB.CheckVersion(ctx); err != nil {
		log.Error("Failed metabase database version check.", zap.Error(err))
		return errs.New("failed metabase version check: %+v", err)
	}

	if err := db.CheckVersion(ctx); err != nil {
		log.Error("Failed satellite database version check.", zap.Error(err))
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()
	return errs.Combine(runError, closeError)
}
