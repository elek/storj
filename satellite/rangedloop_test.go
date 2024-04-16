// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/dbutil/tempdb"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestRangedLoop(t *testing.T) {
	spannerURL := getenv("STORJ_TEST_SPANNER", "STORJ_SPANNER_TEST")
	cockroachDB := getenv("STORJ_TEST_COCKROACH", "STORJ_COCKROACH_TEST")

	require.NotEmpty(t, cockroachDB)
	require.NotEmpty(t, spannerURL)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	schemaSuffix := satellitedbtest.SchemaSuffix()

	tempDB, err := tempdb.OpenUnique(ctx, cockroachDB, "satellite"+schemaSuffix)
	require.NoError(t, err)

	db, err := satellitedbtest.CreateMasterDBOnTopOf(ctx, log, tempDB, "test")
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.Testing().TestMigrateToLatest(ctx))

	metabaseDB, err := metabase.Open(ctx, log, spannerURL, metabase.Config{
		ApplicationName: "test",
	})
	require.NoError(t, err)
	defer metabaseDB.Close()

	require.NoError(t, metabaseDB.TestMigrateToLatest(ctx))

	segments := make([]metabase.RawSegment, 10000)
	for i := range segments {
		obj := metabasetest.RandObjectStream()
		segments[i] = metabasetest.DefaultRawSegment(obj, metabase.SegmentPosition{})
	}

	require.NoError(t, metabaseDB.TestingBatchInsertSegments(ctx, nil, segments))

	satelliteExe := ctx.Compile("storj.io/storj/cmd/satellite")
	satelliteCmd := exec.Command(satelliteExe, "run", "ranged-loop", "--defaults", "release", "--log.level", "debug", "--database", tempDB.ConnStr, "--metainfo.database-url", spannerURL)
	satelliteCmd.Stdout = os.Stdout
	satelliteCmd.Stderr = os.Stderr
	ctx.Go(func() error {
		return satelliteCmd.Run()
	})

	time.Sleep(5 * time.Second)

	require.NoError(t, satelliteCmd.Process.Signal(syscall.SIGTERM))
}

func getenv(priority ...string) string {
	for _, p := range priority {
		v := os.Getenv(p)
		if v != "" {
			return v
		}
	}
	return ""
}
