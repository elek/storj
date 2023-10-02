// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"net"
	"runtime/pprof"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/private/debug"
	"storj.io/storj/private/lifecycle"
	"storj.io/storj/satellite/accounting/nodetally"
	"storj.io/storj/satellite/gc/piecetracker"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
)

// RangedLoop is the satellite ranged loop process.
//
// architecture: Peer
type RangedLoop struct {
	Log *zap.Logger
	DB  DB

	Servers  *lifecycle.Group
	Services *lifecycle.Group

	Audit struct {
		Observer rangedloop.Observer
	}

	Debug struct {
		Listener net.Listener
		Server   *debug.Server
	}

	Metrics struct {
		Observer rangedloop.Observer
	}

	Overlay struct {
		Service *overlay.Service
	}

	Repair struct {
		Observer *checker.Observer
	}

	GracefulExit struct {
		Observer rangedloop.Observer
	}

	Accounting struct {
		NodeTallyObserver *nodetally.Observer
	}

	PieceTracker struct {
		Observer *piecetracker.Observer
	}

	RangedLoop struct {
		Service *rangedloop.Service
	}
}

//
//func setupRepair(config Config) {
//	placement, err := config.Placement.Parse()
//	if err != nil {
//		return nil, err
//	}
//
//	if len(config.Checker.RepairExcludedCountryCodes) == 0 {
//		config.Checker.RepairExcludedCountryCodes = config.Overlay.RepairExcludedCountryCodes
//	}
//}

// NewRangedLoop creates a new satellite ranged loop process.
func NewRangedLoop(log *zap.Logger, db DB, metabaseDB *metabase.DB, config *Config, atomicLogLevel *zap.AtomicLevel) (_ *RangedLoop, err error) {
	peer := &RangedLoop{
		Log: log,
		DB:  db,

		Servers:  lifecycle.NewGroup(log.Named("servers")),
		Services: lifecycle.NewGroup(log.Named("services")),
	}

	{ // setup overlay
		placement, err := config.Placement.Parse()
		if err != nil {
			return nil, err
		}

		peer.Overlay.Service, err = overlay.NewService(peer.Log.Named("overlay"), peer.DB.OverlayCache(), peer.DB.NodeEvents(), placement.CreateFilters, config.Overlay)
		if err != nil {
			return nil, errs.Combine(err, peer.Close())
		}
		peer.Services.Add(lifecycle.Item{
			Name:  "overlay",
			Run:   peer.Overlay.Service.Run,
			Close: peer.Overlay.Service.Close,
		})
	}

	return peer, nil
}

// Run runs satellite ranged loop until it's either closed or it errors.
func (peer *RangedLoop) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group, ctx := errgroup.WithContext(ctx)

	pprof.Do(ctx, pprof.Labels("subsystem", "rangedloop"), func(ctx context.Context) {
		peer.Servers.Run(ctx, group)
		peer.Services.Run(ctx, group)

		pprof.Do(ctx, pprof.Labels("name", "subsystem-wait"), func(ctx context.Context) {
			err = group.Wait()
		})
	})
	return err
}

// Close closes all the resources.
func (peer *RangedLoop) Close() error {
	return errs.Combine(
		peer.Servers.Close(),
		peer.Services.Close(),
	)
}
