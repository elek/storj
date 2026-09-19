// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package consoleext is the extension point for satellite-specific console
// features. Extensions register their own HTTP routes under the console's
// /api/v0 tree and live in their own subpackages, so that adding a feature
// touches the console server in exactly one place.
package consoleext

import (
	"net/http"

	"github.com/gorilla/mux"

	"storj.io/storj/shared/mud"
)

// Deps holds the things an extension needs from the console server that mud
// cannot inject on its own. Everything else - databases, loggers, config -
// should be declared as a constructor argument of the extension instead.
//
// This is a struct rather than a parameter list so that adding a field later
// does not break extensions that do not use it.
type Deps struct {
	// WithAuth wraps a handler so that it only runs for an authenticated
	// session. Inside such a handler, console.GetUser(ctx) returns the user.
	WithAuth func(http.Handler) http.Handler
}

// Extension registers HTTP routes on the console router.
type Extension interface {
	// Name identifies the extension in logs.
	Name() string

	// Register adds the extension's routes to the console router.
	Register(router *mux.Router, deps Deps)
}

// Module is a mud module. It declares the []Extension multibinding that
// extensions append themselves to with mud.Implementation.
func Module(ball *mud.Ball) {
	mud.RegisterImplementation[[]Extension](ball)
}
