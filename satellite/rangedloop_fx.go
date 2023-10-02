// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"go.uber.org/fx"
	"storj.io/storj/satellite/accounting/nodetally"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metrics"
	"storj.io/storj/satellite/repair/checker"
)

var RangedLoopModule = fx.Module("rangedloop",
	fx.Provide(
		fx.Annotate(audit.NewObserver, fx.ResultTags(`group:"observer"`), fx.As(new(rangedloop.Observer))),
		fx.Annotate(metrics.NewObserver, fx.ResultTags(`group:"observer"`), fx.As(new(rangedloop.Observer))),
		fx.Annotate(gracefulexit.NewObserver, fx.ResultTags(`group:"observer"`), fx.As(new(rangedloop.Observer))),
		fx.Annotate(nodetally.NewObserver, fx.ResultTags(`group:"observer"`), fx.As(new(rangedloop.Observer))),
		fx.Annotate(checker.NewObserver, fx.ResultTags(`group:"observer"`), fx.As(new(rangedloop.Observer))),
		fx.Annotate(rangedloop.NewService, fx.OnStart(func(ctx context.Context, service *rangedloop.Service) error {
			return service.Run(ctx)
		})),
		fx.Annotate(rangedloop.NewMetabaseRangeSplitter, fx.As(new(rangedloop.RangeSplitter))),
	))
