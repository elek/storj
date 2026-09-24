// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package dbaccess exposes the satellite database to components which manage
// their own tables and migrations.
package dbaccess

import "storj.io/storj/shared/tagsql"

// Access provides access to the underlying database for independently managed db/migration.
type Access interface {
	// GetDB returns the database to execute queries on.
	GetDB() tagsql.DB
	// GetMigrationDB returns the database to run migration steps on.
	GetMigrationDB() tagsql.DB
}
