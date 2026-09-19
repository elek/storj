// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/statsexport"
)

var _ statsexport.DB = (*statsExport)(nil)

type statsExport struct {
	db *satelliteDB
}

// CountOnlineNodes returns how many participating nodes were contacted successfully
// since onlineSince, and the free disk they reported.
//
// The participation predicate is the one node selection uses, see
// overlaycache.GetAllParticipatingNodes. free_disk is -1 for a node that has never
// reported capacity, which is excluded rather than subtracted from the total.
func (s *statsExport) CountOnlineNodes(ctx context.Context, onlineSince time.Time) (count, freeDisk int64, err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.db.QueryRowContext(ctx, s.db.Rebind(`
		SELECT count(*), COALESCE(sum(CASE WHEN free_disk > 0 THEN free_disk ELSE 0 END), 0)
		FROM nodes
		WHERE disqualified IS NULL
			AND exit_finished_at IS NULL
			AND last_contact_success > ?
	`), onlineSince).Scan(&count, &freeDisk)
	if err != nil {
		return 0, 0, Error.Wrap(err)
	}

	return count, freeDisk, nil
}

// CountActiveUsers returns the number of console accounts in the active state.
func (s *statsExport) CountActiveUsers(ctx context.Context) (count int64, err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.db.QueryRowContext(ctx, s.db.Rebind(`
		SELECT count(*) FROM users WHERE status = ?
	`), console.Active).Scan(&count)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	return count, nil
}

// SumLatestBucketTallies returns the user data recorded by the most recent tally run.
//
// One tally run stamps every bucket it writes with the same interval_start (see
// ProjectAccounting.SaveTallies), so the newest interval_start identifies a whole
// run rather than a single bucket.
//
// Rows written before total_bytes existed hold 0 there and the value in
// inline + remote; every other reader of this table falls back the same way.
func (s *statsExport) SumLatestBucketTallies(ctx context.Context) (total int64, err error) {
	defer mon.Task()(&ctx)(&err)

	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(sum(CASE WHEN total_bytes = 0 THEN inline + remote ELSE total_bytes END), 0)
		FROM bucket_storage_tallies
		WHERE interval_start = (SELECT max(interval_start) FROM bucket_storage_tallies)
	`).Scan(&total)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	return total, nil
}
