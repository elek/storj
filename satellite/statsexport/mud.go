// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package statsexport

import (
	"go.uber.org/zap"

	"storj.io/storj/satellite/overlay"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	// the chore counts nodes as online the same way node selection does, so it
	// borrows the window instead of configuring a second one that could drift.
	mud.Provide[*Chore](ball, func(log *zap.Logger, db DB, overlayConfig overlay.Config, config Config) *Chore {
		return NewChore(log, db, overlayConfig.Node.OnlineWindow, config)
	})
	config.RegisterConfig[Config](ball, "stats-export")
}
