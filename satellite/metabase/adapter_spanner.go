// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"context"
	"github.com/zeebo/errs"
	"log"
	"os"
	"strings"
)

// SpannerConfig includes all the configuration required by using spanner.
type SpannerConfig struct {
	Database string `help:"Database definition for spanner connection in the form  projects/P/instances/I/databases/DB"`
}

// SpannerAdapter implements Adapter for Google Spanner connections..
type SpannerAdapter struct {
	client *spanner.Client
}

// NewSpannerAdapter creates a new Spanner adapter.
func NewSpannerAdapter(ctx context.Context, cfg SpannerConfig) (*SpannerAdapter, error) {
	client, err := spanner.NewClientWithConfig(ctx, cfg.Database,
		spanner.ClientConfig{
			Logger:               log.New(os.Stdout, "spanner", log.LstdFlags),
			SessionPoolConfig:    spanner.DefaultSessionPoolConfig,
			DisableRouteToLeader: false})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &SpannerAdapter{
		client: client,
	}, nil
}

// Close closes the internal client.
func (s *SpannerAdapter) Close() error {
	s.client.Close()
	return nil
}

var _ Adapter = &SpannerAdapter{}

// SpannerMigrationInfo contains information about the current spanner scheme.
type SpannerMigrationInfo struct {
	CurrentVersion int
}

// CreateAdminClient creates and admin client to Spanner db.
func CreateAdminClient(ctx context.Context) (*database.DatabaseAdminClient, error) {
	return database.NewDatabaseAdminClient(ctx)
}

// Migrate migrates spanner scheme to the latest version.
// TODO: right now it drops existing data and re-creates tables.
func Migrate(ctx context.Context, adminClient *database.DatabaseAdminClient, config SpannerConfig) (SpannerMigrationInfo, error) {
	info := SpannerMigrationInfo{
		CurrentVersion: 0,
	}
	req := &databasepb.UpdateDatabaseDdlRequest{
		Database: config.Database,
	}

	for _, ddl := range strings.Split(spannerDDL, ";") {
		if strings.TrimSpace(ddl) != "" {
			req.Statements = append(req.Statements, ddl)
		}
	}
	resp, err := adminClient.UpdateDatabaseDdl(ctx, req)
	if err != nil {
		return info, errs.Wrap(err)
	}

	err = resp.Wait(ctx)
	if err != nil {
		return info, errs.Wrap(err)
	}
	return info, nil
}
