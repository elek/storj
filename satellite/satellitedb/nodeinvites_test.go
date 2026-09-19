// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/nodeinvites"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestNodeInvitesGetCandidates(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		var (
			cache     = db.OverlayCache()
			users     = db.Console().Users()
			tokens    = db.Console().RegistrationTokens()
			invitesDB = db.NodeInvites()

			now          = time.Now()
			selectionCfg = overlay.NodeSelectionConfig{OnlineWindow: 4 * time.Hour}
		)

		// addNodes checks in count nodes for the operator email at the given time and
		// returns their IDs.
		addNodes := func(email string, count int, lastContact time.Time) []storj.NodeID {
			ids := make([]storj.NodeID, 0, count)
			for i := 0; i < count; i++ {
				id := testrand.NodeID()
				require.NoError(t, cache.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
					NodeID:  id,
					IsUp:    true,
					Address: &pb.NodeAddress{Address: "1.2.3.4"},
					Version: &pb.NodeVersion{Version: "v0.0.0"},
					Operator: &pb.NodeOperator{
						Email:  email,
						Wallet: "0x1234567890123456789012345678901234567890",
					},
				}, lastContact, selectionCfg))
				ids = append(ids, id)
			}
			return ids
		}

		addUser := func(email string, status console.UserStatus) *console.User {
			id, err := uuid.New()
			require.NoError(t, err)
			user, err := users.Insert(ctx, &console.User{
				ID:           id,
				FullName:     "Node Operator",
				Email:        email,
				PasswordHash: testrand.Bytes(8),
			})
			require.NoError(t, err)
			require.NoError(t, users.Update(ctx, user.ID, console.UpdateUserRequest{Status: &status}))
			return user
		}

		addToken := func(email string, expiresAt time.Time) *console.RegistrationToken {
			token, err := tokens.CreateWithLimits(ctx, console.CreateRegistrationTokenParams{
				ProjectLimit: 1,
				ExpiresAt:    &expiresAt,
				Partner:      &email,
			})
			require.NoError(t, err)
			return token
		}

		query := nodeinvites.CandidateQuery{
			MinNodes:    3,
			ActiveSince: now.Add(-24 * time.Hour),
			Now:         now,
			MaxAttempts: 3,
			Limit:       100,
		}

		candidateEmails := func(q nodeinvites.CandidateQuery) map[string]int {
			candidates, err := invitesDB.GetCandidates(ctx, q)
			require.NoError(t, err)
			byEmail := make(map[string]int, len(candidates))
			for _, c := range candidates {
				byEmail[c.Email] = c.NodeCount
			}
			return byEmail
		}

		// eligible: enough active nodes, no account, no token.
		addNodes("eligible@storj.test", 3, now)

		// below the threshold.
		addNodes("toofew@storj.test", 2, now)

		// the same operator using different casing is a single pool.
		addNodes("Mixed@Storj.Test", 2, now)
		addNodes("mixed@storj.test", 1, now)

		// nodes that haven't been seen within the active window don't count.
		addNodes("stale@storj.test", 3, now.Add(-48*time.Hour))

		// nodes without an operator email are not a pool.
		addNodes("", 3, now)

		// disqualified and exited nodes don't count.
		dqIDs := addNodes("disqualified@storj.test", 3, now)
		for _, id := range dqIDs[:1] {
			_, err := cache.DisqualifyNode(ctx, id, now, overlay.DisqualificationReasonAuditFailure)
			require.NoError(t, err)
		}
		exitedIDs := addNodes("exited@storj.test", 3, now)
		for _, id := range exitedIDs[:1] {
			_, err := cache.UpdateExitStatus(ctx, &overlay.ExitStatusRequest{
				NodeID:          id,
				ExitInitiatedAt: now,
				ExitFinishedAt:  now,
				ExitSuccess:     true,
			})
			require.NoError(t, err)
		}

		// operators that already have an account, verified or not.
		addNodes("verified@storj.test", 3, now)
		addUser("verified@storj.test", console.Active)
		addNodes("unverified@storj.test", 3, now)
		addUser("unverified@storj.test", console.Inactive)
		// the account was created with different casing than the node reports.
		addNodes("Cased@Storj.Test", 3, now)
		addUser("cased@storj.test", console.Active)

		// an unused token that hasn't expired blocks a new invitation.
		addNodes("active-token@storj.test", 3, now)
		addToken("active-token@storj.test", now.Add(time.Hour))

		// an unused token that expired makes the operator eligible again.
		addNodes("expired-token@storj.test", 3, now)
		addToken("expired-token@storj.test", now.Add(-time.Hour))

		// a token that was used blocks invitations forever, even once expired.
		addNodes("used-token@storj.test", 3, now)
		usedToken := addToken("used-token@storj.test", now.Add(-time.Hour))
		usedBy := addUser("someone-else@storj.test", console.Active)
		require.NoError(t, tokens.UpdateOwner(ctx, usedToken.Secret, usedBy.ID))

		// the attempt cap is reached.
		addNodes("maxed-out@storj.test", 3, now)
		for i := 0; i < 3; i++ {
			addToken("maxed-out@storj.test", now.Add(-time.Hour))
		}

		t.Run("selection", func(t *testing.T) {
			found := candidateEmails(query)

			require.Equal(t, map[string]int{
				"eligible@storj.test":      3,
				"mixed@storj.test":         3,
				"expired-token@storj.test": 3,
			}, found)
		})

		t.Run("one more attempt is allowed below the cap", func(t *testing.T) {
			raised := query
			raised.MaxAttempts = 4

			found := candidateEmails(raised)
			require.Contains(t, found, "maxed-out@storj.test")
		})

		t.Run("largest operators first, limited", func(t *testing.T) {
			addNodes("biggest@storj.test", 6, now)
			defer func() {
				// keep the rest of the subtests unaffected.
				addToken("biggest@storj.test", now.Add(time.Hour))
			}()

			candidates, err := invitesDB.GetCandidates(ctx, nodeinvites.CandidateQuery{
				MinNodes:    query.MinNodes,
				ActiveSince: query.ActiveSince,
				Now:         query.Now,
				MaxAttempts: query.MaxAttempts,
				Limit:       1,
			})
			require.NoError(t, err)
			require.Len(t, candidates, 1)
			require.Equal(t, "biggest@storj.test", candidates[0].Email)
			require.Equal(t, 6, candidates[0].NodeCount)
		})

		t.Run("empty parameters return nothing", func(t *testing.T) {
			for _, q := range []nodeinvites.CandidateQuery{
				{MinNodes: 0, MaxAttempts: 3, Limit: 10},
				{MinNodes: 3, MaxAttempts: 0, Limit: 10},
				{MinNodes: 3, MaxAttempts: 3, Limit: 0},
			} {
				q.ActiveSince, q.Now = query.ActiveSince, query.Now
				candidates, err := invitesDB.GetCandidates(ctx, q)
				require.NoError(t, err)
				require.Empty(t, candidates)
			}
		})
	})
}
