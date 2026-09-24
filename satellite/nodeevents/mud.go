// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "node-events")
	mud.Provide[*MockNotifier](ball, NewMockNotifier)
	mud.Provide[*CustomerioNotifier](ball, NewCustomerioNotifier)
	mud.RegisterInterfaceImplementation[Notifier, *CustomerioNotifier](ball)
	// Notifiers which should be used together join the []Notifier multibinding,
	// which MultiNotifier fans out to.
	mud.Implementation[[]Notifier, *CustomerioNotifier](ball)
	mud.Provide[*MultiNotifier](ball, NewMultiNotifier)
	mud.View[*Config, CustomerioConfig](ball, func(cfg *Config) CustomerioConfig {
		return cfg.Customerio
	})
}
