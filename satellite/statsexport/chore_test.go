// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package statsexport_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/satellite/statsexport"
)

const onlineWindow = 4 * time.Hour

// fakeDB returns canned numbers, and can be told to fail.
type fakeDB struct {
	nodes       int64
	freeDisk    int64
	users       int64
	dataStored  int64
	err         error
	onlineSince time.Time
}

func (f *fakeDB) CountOnlineNodes(_ context.Context, onlineSince time.Time) (int64, int64, error) {
	f.onlineSince = onlineSince
	return f.nodes, f.freeDisk, f.err
}

func (f *fakeDB) CountActiveUsers(context.Context) (int64, error) {
	return f.users, f.err
}

func (f *fakeDB) SumLatestBucketTallies(context.Context) (int64, error) {
	return f.dataStored, f.err
}

// readStats reads and decodes the exported file.
func readStats(t *testing.T, path string) statsexport.Stats {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var stats statsexport.Stats
	require.NoError(t, json.Unmarshal(data, &stats))
	return stats
}

func TestChoreWritesStats(t *testing.T) {
	ctx := testcontext.New(t)
	path := filepath.Join(t.TempDir(), "stats.json")

	db := &fakeDB{nodes: 24000, freeDisk: 1 << 50, users: 42, dataStored: 1 << 40}
	chore := statsexport.NewChore(zaptest.NewLogger(t), db, onlineWindow, statsexport.Config{
		Path:     path,
		Interval: time.Hour,
	})

	now := time.Date(2026, 9, 19, 20, 0, 0, 0, time.UTC)
	chore.TestSetNow(func() time.Time { return now })

	require.NoError(t, chore.RunOnce(ctx))

	stats := readStats(t, path)
	require.Equal(t, now, stats.Updated.UTC())
	require.EqualValues(t, 24000, stats.NodesOnline)
	require.EqualValues(t, 42, stats.Users)
	require.EqualValues(t, 1<<40, stats.DataStoredBytes)
	require.EqualValues(t, 1<<50, stats.FreeSpaceBytes)

	// nodes are counted as online using the overlay window.
	require.Equal(t, now.Add(-onlineWindow), db.onlineSince)
}

func TestChoreFieldNames(t *testing.T) {
	ctx := testcontext.New(t)
	path := filepath.Join(t.TempDir(), "stats.json")

	chore := statsexport.NewChore(zaptest.NewLogger(t), &fakeDB{}, onlineWindow, statsexport.Config{
		Path:     path,
		Interval: time.Hour,
	})
	chore.TestSetNow(func() time.Time {
		return time.Date(2026, 9, 19, 20, 0, 0, 0, time.UTC)
	})

	require.NoError(t, chore.RunOnce(ctx))

	// the file is a published format, so pin the exact keys and the timestamp layout.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"updated": "2026-09-19T20:00:00Z",
		"nodes_online": 0,
		"users": 0,
		"data_stored_bytes": 0,
		"free_space_bytes": 0
	}`, string(data))
}

func TestChoreOverwrites(t *testing.T) {
	ctx := testcontext.New(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")

	db := &fakeDB{nodes: 1}
	chore := statsexport.NewChore(zaptest.NewLogger(t), db, onlineWindow, statsexport.Config{
		Path:     path,
		Interval: time.Hour,
	})

	require.NoError(t, chore.RunOnce(ctx))
	db.nodes = 2
	require.NoError(t, chore.RunOnce(ctx))

	require.EqualValues(t, 2, readStats(t, path).NodesOnline)

	// the rename leaves nothing behind, so a directory listing stays servable.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "stats.json", entries[0].Name())
}

func TestChoreQueryFailureKeepsPreviousFile(t *testing.T) {
	ctx := testcontext.New(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")

	db := &fakeDB{nodes: 7}
	chore := statsexport.NewChore(zaptest.NewLogger(t), db, onlineWindow, statsexport.Config{
		Path:     path,
		Interval: time.Hour,
	})

	require.NoError(t, chore.RunOnce(ctx))

	db.err = errors.New("database is down")
	require.Error(t, chore.RunOnce(ctx))

	// a failed run must not truncate or remove what a reader is already serving.
	require.EqualValues(t, 7, readStats(t, path).NodesOnline)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestChoreDisabled(t *testing.T) {
	ctx := testcontext.New(t)

	db := &fakeDB{err: errors.New("should not be queried")}
	chore := statsexport.NewChore(zaptest.NewLogger(t), db, onlineWindow, statsexport.Config{
		Path:     "",
		Interval: time.Hour,
	})

	// Run returns immediately without touching the database.
	require.NoError(t, chore.Run(ctx))
	require.True(t, db.onlineSince.IsZero())

	// the peer still closes it, on a cycle that was never started.
	require.NoError(t, chore.Close())
}

func TestChoreWithRealDB(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		path := filepath.Join(t.TempDir(), "stats.json")

		now := time.Now()
		selectionCfg := overlay.NodeSelectionConfig{OnlineWindow: onlineWindow}

		// two online nodes with capacity, one that has not been seen in a week.
		for _, node := range []struct {
			freeDisk    int64
			lastContact time.Time
		}{
			{freeDisk: 300, lastContact: now},
			{freeDisk: 700, lastContact: now.Add(-time.Hour)},
			{freeDisk: 999, lastContact: now.Add(-7 * 24 * time.Hour)},
		} {
			require.NoError(t, db.OverlayCache().UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
				NodeID:   testrand.NodeID(),
				IsUp:     true,
				Address:  &pb.NodeAddress{Address: "1.2.3.4"},
				Version:  &pb.NodeVersion{Version: "v0.0.0"},
				Capacity: &pb.NodeCapacity{FreeDisk: node.freeDisk},
				Operator: &pb.NodeOperator{
					Email:  "operator@storj.test",
					Wallet: "0x1234567890123456789012345678901234567890",
				},
			}, node.lastContact, selectionCfg))
		}

		// one active user, one that never activated.
		for i, status := range []console.UserStatus{console.Active, console.Inactive} {
			id, err := uuid.New()
			require.NoError(t, err)
			user, err := db.Console().Users().Insert(ctx, &console.User{
				ID:           id,
				FullName:     "Test User",
				Email:        fmt.Sprintf("user%d@storj.test", i),
				PasswordHash: testrand.Bytes(8),
			})
			require.NoError(t, err)
			require.NoError(t, db.Console().Users().Update(ctx, user.ID,
				console.UpdateUserRequest{Status: &status}))
		}

		projectID := testrand.UUID()
		location := func(name string) metabase.BucketLocation {
			return metabase.BucketLocation{ProjectID: projectID, BucketName: metabase.BucketName(name)}
		}

		// an older tally run that must not be counted, then the current one.
		require.NoError(t, db.ProjectAccounting().SaveTallies(ctx, now.Add(-time.Hour),
			map[metabase.BucketLocation]*accounting.BucketTally{
				location("one"): {BucketLocation: location("one"), TotalBytes: 100000},
			}))
		require.NoError(t, db.ProjectAccounting().SaveTallies(ctx, now,
			map[metabase.BucketLocation]*accounting.BucketTally{
				location("one"): {BucketLocation: location("one"), TotalBytes: 1000},
				location("two"): {BucketLocation: location("two"), TotalBytes: 2000},
			}))

		chore := statsexport.NewChore(zaptest.NewLogger(t), db.StatsExport(), onlineWindow,
			statsexport.Config{Path: path, Interval: time.Hour})

		require.NoError(t, chore.RunOnce(ctx))

		stats := readStats(t, path)
		require.EqualValues(t, 2, stats.NodesOnline)
		require.EqualValues(t, 1000, stats.FreeSpaceBytes)
		require.EqualValues(t, 1, stats.Users)
		require.EqualValues(t, 3000, stats.DataStoredBytes)
		require.WithinDuration(t, time.Now(), stats.Updated, time.Minute)
	})
}
