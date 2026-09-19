// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/console/consoleext"
	consoleextnodes "storj.io/storj/satellite/console/consoleext/nodes"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/shared/mud"
)

// Console extensions reach the web server through a mud multibinding, which
// silently yields an empty slice if an extension forgets to register itself.
// Nothing else in the build would notice: the server starts, and the routes
// are just absent.
func TestConsoleExtensionsAreWired(t *testing.T) {
	ball := mud.NewBall()

	// these are provided by the CLI environment
	mud.Provide[*modular.StopTrigger](ball, func() *modular.StopTrigger {
		return &modular.StopTrigger{}
	})
	mud.Provide[*cli.ConfigDir](ball, func() *cli.ConfigDir {
		return &cli.ConfigDir{Dir: t.TempDir()}
	})
	mud.View[*cli.ConfigDir, cli.ConfigDir](ball, mud.Dereference)

	Module(ball)

	components := mud.FindSelectedWithDependencies(ball, mud.Select[*consoleweb.Server](ball))

	var targets []reflect.Type
	for _, c := range components {
		targets = append(targets, c.GetTarget())
	}

	require.Contains(t, targets, reflect.TypeFor[[]consoleext.Extension](),
		"console server should depend on the extension list")
	require.Contains(t, targets, reflect.TypeFor[*consoleextnodes.Extension](),
		"nodes extension should be reachable from the console server")
}
