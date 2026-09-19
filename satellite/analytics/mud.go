// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package analytics

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "analytics")
	mud.Provide[*NoopService](ball, NewNoopService)

	// *SegmentService is provided by satellite.Module, as it depends on consoleweb.Config.
	mud.RegisterInterfaceImplementation[Service, *SegmentService](ball)
	mud.RegisterInterfaceImplementation[FreezeTracker, Service](ball)
}
