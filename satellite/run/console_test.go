// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/console/consoleext"
	consoleextnodes "storj.io/storj/satellite/console/consoleext/nodes"
	consoleextnotifications "storj.io/storj/satellite/console/consoleext/notifications"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/satellite/telegram"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/shared/mud"
)

// Console extensions reach the web server through a mud multibinding, which
// silently yields an empty slice if an extension forgets to register itself.
// Nothing else in the build would notice: the server starts, and the routes
// are just absent.
func TestConsoleExtensionsAreWired(t *testing.T) {
	ball := testBall(t)

	targets := selectedTargets(ball, mud.Select[*consoleweb.Server](ball))

	require.Contains(t, targets, reflect.TypeFor[[]consoleext.Extension](),
		"console server should depend on the extension list")
	require.Contains(t, targets, reflect.TypeFor[*consoleextnodes.Extension](),
		"nodes extension should be reachable from the console server")
	require.Contains(t, targets, reflect.TypeFor[*consoleextnotifications.Extension](),
		"notifications extension should be reachable from the console server")
}

// The node events notifier is swapped with --components. Email stays the
// default; Telegram can replace it, or be used together with it.
func TestNodeEventsNotifierSelection(t *testing.T) {
	chore := func(ball *mud.Ball) []reflect.Type {
		return selectedTargets(ball, mud.Select[*nodeevents.Chore](ball))
	}
	email := reflect.TypeFor[*nodeevents.CustomerioNotifier]()
	tg := reflect.TypeFor[*telegram.Notifier]()

	t.Run("default", func(t *testing.T) {
		targets := chore(testBall(t))
		require.Contains(t, targets, email)
		require.NotContains(t, targets, tg)
	})

	t.Run("telegram only", func(t *testing.T) {
		ball := testBall(t)
		modular.CreateSelectorFromString(ball, "nodeevents.Notifier=telegram.Notifier")
		targets := chore(ball)
		require.Contains(t, targets, tg)
		require.NotContains(t, targets, email)
		require.Contains(t, targets, reflect.TypeFor[telegram.Persistence](),
			"the chats should be read from the telegram_chats table")
	})

	t.Run("email and telegram", func(t *testing.T) {
		ball := testBall(t)
		modular.CreateSelectorFromString(ball, "nodeevents.Notifier=nodeevents.MultiNotifier")
		targets := chore(ball)
		require.Contains(t, targets, reflect.TypeFor[*nodeevents.MultiNotifier]())
		require.Contains(t, targets, tg)
		require.Contains(t, targets, email)
	})
}

func testBall(t *testing.T) *mud.Ball {
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
	return ball
}

func selectedTargets(ball *mud.Ball, selector mud.ComponentSelector) (targets []reflect.Type) {
	for _, c := range mud.FindSelectedWithDependencies(ball, selector) {
		targets = append(targets, c.GetTarget())
	}
	return targets
}
