// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package planaccess_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleext/nodes"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/planaccess"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/mudplanet"
	"storj.io/storj/shared/mudplanet/satellitetest"
)

const ownPlacement = storj.PlacementConstraint(12)

func TestChore(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		satelliteID := testrand.NodeID()
		now := time.Now()

		withPlacement := addUser(ctx, t, db, "with-placement@storj.test", console.Active)
		withoutPlacement := addUser(ctx, t, db, "without-placement@storj.test", console.Active)
		inactive := addUser(ctx, t, db, "inactive@storj.test", console.Inactive)
		fewNodes := addUser(ctx, t, db, "few-nodes@storj.test", console.Active)

		// the default placement project already exists, and has to be reused.
		existing, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:             "existing",
			OwnerID:          withPlacement.ID,
			DefaultPlacement: storj.DefaultPlacement,
		})
		require.NoError(t, err)

		first := addNode(ctx, t, db, 1*memory.GB, now)
		second := addNode(ctx, t, db, 2*memory.GB, now)
		// the third node reports no free space, so it only counts towards the minimum.
		setOwner(ctx, t, db, satelliteID, withPlacement.ID, first, second, addNode(ctx, t, db, 0, now))

		// a suspended node doesn't count towards the minimum.
		suspended := addNode(ctx, t, db, memory.GB, now)
		require.NoError(t, db.OverlayCache().TestSuspendNodeOffline(ctx, suspended, now))
		setOwner(ctx, t, db, satelliteID, fewNodes.ID, addNode(ctx, t, db, memory.GB, now), addNode(ctx, t, db, memory.GB, now), suspended)

		setOwner(ctx, t, db, satelliteID, withoutPlacement.ID,
			addNode(ctx, t, db, memory.GB, now), addNode(ctx, t, db, memory.GB, now), addNode(ctx, t, db, memory.GB, now))
		setOwner(ctx, t, db, satelliteID, inactive.ID, addNode(ctx, t, db, memory.GB, now))

		// owner tags signed by somebody else are ignored.
		setOwner(ctx, t, db, testrand.NodeID(), withoutPlacement.ID, addNode(ctx, t, db, memory.GB, now))
		// nodes without an owner are ignored.
		addNode(ctx, t, db, memory.GB, now)

		// tallies are saved in byte-hours, 2 hours apart: the nodes store 3 GB and 1 GB.
		tallyTime := now.Add(-time.Hour)
		accountingDB := db.StoragenodeAccounting()
		require.NoError(t, accountingDB.SaveTallies(ctx, tallyTime.Add(-2*time.Hour),
			[]storj.NodeID{first, second}, []float64{1, 1}))
		require.NoError(t, accountingDB.SaveTallies(ctx, tallyTime,
			[]storj.NodeID{first, second}, []float64{float64(2 * 3 * memory.GB), float64(2 * memory.GB)}))

		chore := planaccess.NewChore(zaptest.NewLogger(t), db.OverlayCache(), db.Console(), accountingDB,
			nodeselection.PlacementDefinitions{
				storj.DefaultPlacement: {ID: storj.DefaultPlacement, Name: "global"},
				ownPlacement:           {ID: ownPlacement, Name: string(nodes.EncodeOwner(withPlacement.ID))},
				ownPlacement + 1:       {ID: ownPlacement + 1, Name: string(nodes.EncodeOwner(fewNodes.ID))},
			},
			satelliteID,
			planaccess.Config{Interval: time.Hour, OnlineWindow: 4 * time.Hour, TallyLookback: 48 * time.Hour, MinNodes: 3},
		)

		check := func() {
			projects, err := db.Console().Projects().GetOwnActive(ctx, withPlacement.ID)
			require.NoError(t, err)
			require.Len(t, projects, 2)

			for _, project := range projects {
				switch project.DefaultPlacement {
				case storj.DefaultPlacement:
					require.Equal(t, existing.ID, project.ID)
					requireInt64(t, 0, project.SegmentLimit)
					requireInt(t, 0, project.RateLimitPut)
					requireInt(t, 0, project.RateLimitGet)
				case ownPlacement:
					require.NotNil(t, project.StorageLimit)
					// (free 1 GB + 2 GB, used 3 GB + 1 GB) * 0.95
					require.Equal(t, int64(float64(7*memory.GB)*0.95), project.StorageLimit.Int64())
					requireInt(t, 10000, project.RateLimitPut)
					requireInt(t, 10000, project.RateLimitGet)

					members, err := db.Console().ProjectMembers().GetByMemberID(ctx, withPlacement.ID)
					require.NoError(t, err)
					var isMember bool
					for _, member := range members {
						isMember = isMember || member.ProjectID == project.ID
					}
					require.True(t, isMember, "owner is not a member of the created project")
				default:
					t.Fatalf("unexpected placement %d", project.DefaultPlacement)
				}
			}

			// without a placement, or with too few active nodes, only the default
			// placement project is ensured.
			for _, owner := range []uuid.UUID{withoutPlacement.ID, fewNodes.ID} {
				projects, err = db.Console().Projects().GetOwnActive(ctx, owner)
				require.NoError(t, err)
				require.Len(t, projects, 1)
				require.Equal(t, storj.DefaultPlacement, projects[0].DefaultPlacement)
				requireInt64(t, 0, projects[0].SegmentLimit)
				requireInt(t, 0, projects[0].RateLimitPut)
				requireInt(t, 0, projects[0].RateLimitGet)
			}

			projects, err = db.Console().Projects().GetOwnActive(ctx, inactive.ID)
			require.NoError(t, err)
			require.Empty(t, projects)
		}

		require.NoError(t, chore.RunOnce(ctx))
		check()

		// a second run doesn't create more projects.
		require.NoError(t, chore.RunOnce(ctx))
		check()
	})
}

func TestChoreMudWiring(t *testing.T) {
	mudplanet.Run(t, satellitetest.WithDB(
		mudplanet.NewComponent("satellite", satellitetest.Satellite,
			mudplanet.WithRunning[*planaccess.Chore](),
		),
	), func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		chore := mudplanet.FindFirst[*planaccess.Chore](t, run, "satellite", 0)
		require.NotNil(t, chore)
	})
}

func addUser(ctx context.Context, t *testing.T, db satellite.DB, email string, status console.UserStatus) *console.User {
	t.Helper()
	user, err := db.Console().Users().Insert(ctx, &console.User{
		ID:           testrand.UUID(),
		FullName:     "Operator",
		Email:        email,
		PasswordHash: []byte("hash"),
	})
	require.NoError(t, err)
	require.NoError(t, db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Status: &status}))
	user.Status = status
	return user
}

func addNode(ctx context.Context, t *testing.T, db satellite.DB, freeDisk memory.Size, lastContact time.Time) storj.NodeID {
	t.Helper()
	id := testrand.NodeID()
	require.NoError(t, db.OverlayCache().UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
		NodeID:   id,
		IsUp:     true,
		Address:  &pb.NodeAddress{Address: "1.2.3.4"},
		Version:  &pb.NodeVersion{Version: "v0.0.0"},
		Operator: &pb.NodeOperator{Email: "operator@storj.test"},
		Capacity: &pb.NodeCapacity{FreeDisk: freeDisk.Int64()},
	}, lastContact, overlay.NodeSelectionConfig{OnlineWindow: 4 * time.Hour}))
	return id
}

func setOwner(ctx context.Context, t *testing.T, db satellite.DB, signer storj.NodeID, owner uuid.UUID, nodeIDs ...storj.NodeID) {
	t.Helper()
	var tags nodeselection.NodeTags
	for _, id := range nodeIDs {
		tags = append(tags, nodeselection.NodeTag{
			NodeID:   id,
			Name:     nodes.OwnerTagName,
			Value:    nodes.EncodeOwner(owner),
			SignedAt: time.Now(),
			Signer:   signer,
		})
	}
	require.NoError(t, db.OverlayCache().UpdateNodeTags(ctx, tags))
}

func requireInt64(t *testing.T, expected int64, actual *int64) {
	t.Helper()
	require.NotNil(t, actual)
	require.Equal(t, expected, *actual)
}

func requireInt(t *testing.T, expected int, actual *int) {
	t.Helper()
	require.NotNil(t, actual)
	require.Equal(t, expected, *actual)
}
