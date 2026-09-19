// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package statsexport writes network-wide statistics to a JSON file.
package statsexport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
)

var (
	// Error is the standard error class for the stats export chore.
	Error = errs.Class("stats export")
	mon   = monkit.Package()
)

// Config holds configurable values for the stats export chore.
type Config struct {
	Path     string        `help:"file to write the network statistics to, disabled when empty" default:""`
	Interval time.Duration `help:"how often to refresh the exported statistics" default:"10m" devDefault:"30s"`
}

// Stats is the exported snapshot, and defines the format of the JSON file.
type Stats struct {
	// Updated is when the snapshot was written, not when the underlying numbers were computed.
	Updated time.Time `json:"updated"`
	// NodesOnline is the number of participating nodes contacted within the online window.
	NodesOnline int64 `json:"nodes_online"`
	// Users is the number of active console accounts.
	Users int64 `json:"users"`
	// DataStoredBytes is the user data stored on the network, before erasure coding expansion.
	DataStoredBytes int64 `json:"data_stored_bytes"`
	// FreeSpaceBytes is the free disk reported by the online nodes.
	FreeSpaceBytes int64 `json:"free_space_bytes"`
}

// DB reads the network-wide numbers that get exported.
//
// architecture: Database
type DB interface {
	// CountOnlineNodes returns the number of participating nodes contacted since
	// onlineSince, and the free disk they reported.
	CountOnlineNodes(ctx context.Context, onlineSince time.Time) (count, freeDisk int64, err error)
	// CountActiveUsers returns the number of active console accounts.
	CountActiveUsers(ctx context.Context) (int64, error)
	// SumLatestBucketTallies returns the user data recorded by the most recent tally run.
	SumLatestBucketTallies(ctx context.Context) (int64, error)
}

// Chore writes network-wide statistics to a JSON file on an interval.
//
// It is disabled, and does not query anything, unless a path is configured.
type Chore struct {
	log          *zap.Logger
	db           DB
	onlineWindow time.Duration
	config       Config
	nowFn        func() time.Time
	Loop         *sync2.Cycle
}

// NewChore creates a new Chore.
func NewChore(log *zap.Logger, db DB, onlineWindow time.Duration, config Config) *Chore {
	return &Chore{
		log:          log,
		db:           db,
		onlineWindow: onlineWindow,
		config:       config,
		nowFn:        time.Now,
		Loop:         sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if chore.config.Path == "" {
		chore.log.Debug("stats export is disabled, no path configured")
		return nil
	}

	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		if err := chore.RunOnce(ctx); err != nil {
			chore.log.Error("exporting network statistics", zap.Error(err))
		}
		return nil
	})
}

// Close closes the chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}

// RunOnce collects the statistics and writes them to the configured path.
func (chore *Chore) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	stats, err := chore.collect(ctx)
	if err != nil {
		return err
	}

	return chore.write(chore.config.Path, stats)
}

// collect reads the current statistics.
func (chore *Chore) collect(ctx context.Context) (_ Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	now := chore.nowFn()

	nodesOnline, freeSpace, err := chore.db.CountOnlineNodes(ctx, now.Add(-chore.onlineWindow))
	if err != nil {
		return Stats{}, Error.Wrap(err)
	}

	users, err := chore.db.CountActiveUsers(ctx)
	if err != nil {
		return Stats{}, Error.Wrap(err)
	}

	dataStored, err := chore.db.SumLatestBucketTallies(ctx)
	if err != nil {
		return Stats{}, Error.Wrap(err)
	}

	mon.IntVal("exported_nodes_online").Observe(nodesOnline)
	mon.IntVal("exported_users").Observe(users)
	mon.IntVal("exported_data_stored_bytes").Observe(dataStored)
	mon.IntVal("exported_free_space_bytes").Observe(freeSpace)

	return Stats{
		Updated:         now.UTC(),
		NodesOnline:     nodesOnline,
		Users:           users,
		DataStoredBytes: dataStored,
		FreeSpaceBytes:  freeSpace,
	}, nil
}

// write marshals the stats and puts them at path.
//
// The file is written to a temporary name and renamed into place, so that a
// reader polling the path never sees a partially written file. The temporary
// file is created next to the target rather than in TMPDIR to keep the rename
// within one filesystem, where it is atomic.
func (chore *Chore) write(path string, stats Stats) (err error) {
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return Error.Wrap(err)
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		if err != nil {
			err = errs.Combine(err, Error.Wrap(os.Remove(temp.Name())))
		}
	}()

	if _, err = temp.Write(data); err != nil {
		return Error.Wrap(errs.Combine(err, temp.Close()))
	}
	if err = temp.Close(); err != nil {
		return Error.Wrap(err)
	}

	// CreateTemp makes the file 0600, but this is a published snapshot with
	// nothing sensitive in it and whatever serves it is a different user.
	if err = os.Chmod(temp.Name(), 0644); err != nil {
		return Error.Wrap(err)
	}

	return Error.Wrap(os.Rename(temp.Name(), path))
}

// TestSetNow allows tests to have the chore act as if the current time is whatever they want.
func (chore *Chore) TestSetNow(nowFn func() time.Time) {
	chore.nowFn = nowFn
}
