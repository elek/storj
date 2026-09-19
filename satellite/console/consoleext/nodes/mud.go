// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodes

import (
	"storj.io/storj/satellite/console/consoleext"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*Extension](ball, New)
	mud.Implementation[[]consoleext.Extension, *Extension](ball)
}
