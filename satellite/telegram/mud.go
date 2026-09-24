// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package telegram

import (
	"storj.io/storj/satellite/nodeevents"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
//
// The notifier is not used unless it is selected, either instead of the email
// notifier:
//
//	--components=nodeevents.Notifier=telegram.Notifier
//
// or together with it:
//
//	--components=nodeevents.Notifier=nodeevents.MultiNotifier
//
// The bot, which connects chats to console users, has to run in exactly one
// process, for example --components=telegram.Bot.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "telegram")
	mud.Provide[*Migration](ball, NewMigration)
	mud.Provide[Persistence](ball, NewDB)
	mud.Provide[*Links](ball, NewLinks)
	mud.Provide[*Service](ball, NewService)
	mud.Provide[*Bot](ball, NewBot)
	mud.Provide[*Notifier](ball, NewNotifier)
	mud.Implementation[[]nodeevents.Notifier, *Notifier](ball)
}
