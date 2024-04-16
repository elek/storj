package metabase

import (
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"context"
	"github.com/stretchr/testify/require"
	"os"
	"storj.io/storj/private/mud"
	"storj.io/storj/private/mud/mudtest"
	"testing"
)

func TestMigrate(t *testing.T) {
	spanner := os.Getenv("STORJ_TEST_SPANNER")
	if spanner == "" || spanner == "omit" {
		t.Skip("Spannert test is not enabled")
		return
	}
	mudtest.Run[SpannerMigrationInfo](t, func(ball *mud.Ball) {
		mud.Supply[SpannerConfig](ball, SpannerConfig{
			Database: spanner,
		})
		mud.Provide[*database.DatabaseAdminClient](ball, CreateAdminClient)
		mud.Provide[SpannerMigrationInfo](ball, Migrate)
	}, func(ctx context.Context, t *testing.T, info SpannerMigrationInfo) {
		require.Equal(t, 0, info.CurrentVersion)
	})

}
