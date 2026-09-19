// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/satellite/nodeinvites"
	"storj.io/storj/shared/tagsql"
)

var _ nodeinvites.DB = (*nodeInvites)(nil)

type nodeInvites struct {
	db *satelliteDB
}

// GetCandidates returns the node operators that should receive an invitation.
//
// An operator qualifies when they run at least MinNodes nodes that were contacted
// successfully since ActiveSince, have no console account, have no active
// registration token, and have received fewer than MaxAttempts invitations.
//
// Operator emails are compared case insensitively, the same way the console nodes
// view resolves an account to its nodes. Returned emails are lowercased.
//
// N.B. registration_tokens.partner holds the operator email for tokens created by
// the node invites chore. See nodeinvites.Chore for why.
func (n *nodeInvites) GetCandidates(ctx context.Context, query nodeinvites.CandidateQuery) (_ []nodeinvites.Candidate, err error) {
	defer mon.Task()(&ctx)(&err)

	candidates := make([]nodeinvites.Candidate, 0, query.Limit)

	if query.MinNodes <= 0 || query.MaxAttempts <= 0 || query.Limit <= 0 {
		return candidates, nil
	}

	err = withRows(n.db.QueryContext(ctx, n.db.Rebind(`
		WITH pools AS (
			SELECT LOWER(email) AS operator_email, count(*) AS node_count
			FROM nodes
			WHERE last_contact_success > ?
				AND disqualified IS NULL
				AND exit_finished_at IS NULL
				AND email <> ''
			GROUP BY LOWER(email)
			HAVING count(*) >= ?
		)
		SELECT p.operator_email, p.node_count
		FROM pools p
		WHERE NOT EXISTS (
				SELECT 1 FROM users u
				WHERE u.normalized_email = UPPER(p.operator_email)
			)
			AND NOT EXISTS (
				SELECT 1 FROM registration_tokens t
				WHERE t.partner = p.operator_email
					AND (t.owner_id IS NOT NULL OR t.expires_at IS NULL OR t.expires_at > ?)
			)
			AND (
				SELECT count(*) FROM registration_tokens t2
				WHERE t2.partner = p.operator_email
			) < ?
		ORDER BY p.node_count DESC, p.operator_email
		LIMIT ?
	`), query.ActiveSince, query.MinNodes, query.Now, query.MaxAttempts, query.Limit))(func(rows tagsql.Rows) error {
		for rows.Next() {
			var candidate nodeinvites.Candidate
			if err := rows.Scan(&candidate.Email, &candidate.NodeCount); err != nil {
				return err
			}
			candidates = append(candidates, candidate)
		}
		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return candidates, nil
}
