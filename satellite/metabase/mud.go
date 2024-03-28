// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"crypto/rand"
	_ "embed"
	"strings"

	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/mud"
)

//go:embed adapter_spanner_scheme.sql
var spannerDDL string

// SpannerTestModule adds all the required dependencies for Spanner migration and adapter.
func SpannerTestModule(ball *mud.Ball, spannerConnection string) {
	mud.Provide[*SpannerAdapter](ball, NewSpannerAdapter)
	mud.Implementation[[]Adapter, *SpannerAdapter](ball)
	mud.RemoveTag[*SpannerAdapter, mud.Optional](ball)
	mud.Provide[SpannerTestDatabase](ball, func(ctx context.Context, logger *zap.Logger) (SpannerTestDatabase, error) {
		return NewTestDatabase(ctx, logger, spannerConnection)
	})
	mud.Provide[SpannerConfig](ball, NewTestSpannerConfig)
}

// SpannerTestDatabase manages Spanner database and migration for tests.
type SpannerTestDatabase struct {
	Database string
	client   *database.DatabaseAdminClient
}

// NewTestDatabase creates the database (=creates / migrates the database).
func NewTestDatabase(ctx context.Context, logger *zap.Logger, spannerConnection string) (SpannerTestDatabase, error) {
	data := make([]byte, 8)
	_, err := rand.Read(data)
	if err != nil {
		return SpannerTestDatabase{}, errs.Wrap(err)
	}

	adminClient, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return SpannerTestDatabase{}, errs.Wrap(err)
	}

	logger.Info("Creating temporary spanner database", zap.String("db", spannerConnection))

	parts := strings.Split(spannerConnection, "/")
	instance := strings.Join(parts[0:4], "/")
	name := parts[len(parts)-1]
	req := &databasepb.CreateDatabaseRequest{
		Parent:          instance,
		DatabaseDialect: databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL,
		CreateStatement: "CREATE DATABASE " + name,
	}

	for _, ddl := range strings.Split(spannerDDL, ";") {
		if strings.TrimSpace(ddl) != "" {
			req.ExtraStatements = append(req.ExtraStatements, ddl)
		}
	}
	ddl, err := adminClient.CreateDatabase(ctx, req)
	if err != nil {
		return SpannerTestDatabase{}, errs.Wrap(err)
	}
	_, err = ddl.Wait(ctx)
	if err != nil {
		return SpannerTestDatabase{}, errs.Wrap(err)
	}
	return SpannerTestDatabase{
		Database: spannerConnection,
		client:   adminClient,
	}, nil
}

// Close drops the temporary test database.
func (d SpannerTestDatabase) Close(ctx context.Context) error {
	err := d.client.DropDatabase(ctx, &databasepb.DropDatabaseRequest{
		Database: d.Database,
	})
	return errs.Combine(err, d.client.Close())
}

// NewTestSpannerConfig creates SpannerConfig for testing.
func NewTestSpannerConfig(database SpannerTestDatabase) SpannerConfig {
	return SpannerConfig{
		Database: database.Database,
	}
}
