// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package telegram

import (
	"storj.io/storj/private/migrate"
	"storj.io/storj/satellite/satellitedb/dbaccess"
)

// Migration creates the tables of the Telegram notifications. It shares the
// versions table of the satellite database, under its own namespace, so its
// steps are independent of the numbered satellitedb migrations.
//
// The migration is a named field, as mud would treat the Run method of an
// embedded one as the lifecycle hook of the component.
type Migration struct {
	Migration migrate.Migration
}

// NewMigration creates the migration of the Telegram tables.
func NewMigration(db dbaccess.Access) *Migration {
	migrationDB := db.GetMigrationDB()
	return &Migration{
		Migration: migrate.Migration{
			Table:     "versions",
			Namespace: "telegram",
			Steps: []*migrate.Step{
				{
					DB:          &migrationDB,
					Description: "Create telegram_chats table",
					Version:     0,
					Action: migrate.SQL{`
						CREATE TABLE IF NOT EXISTS telegram_chats (
							user_id bytea NOT NULL,
							chat_id bigint NOT NULL,
							created_at timestamp with time zone NOT NULL DEFAULT current_timestamp,
							PRIMARY KEY ( user_id )
						)`,
					},
				},
			},
		},
	}
}
