// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package planaccess_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"gopkg.in/yaml.v3"

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
					require.Equal(t, "Disabled due to not enough SNOs", project.Description)
					requireInt64(t, 0, project.SegmentLimit)
					requireInt(t, 0, project.RateLimitPut)
					requireInt(t, 0, project.RateLimitGet)
				case ownPlacement:
					require.Equal(t, "Stored on own storagenodes", project.Description)
					requireInt64(t, 1_000_000_000, project.SegmentLimit)
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

			// without a placement, only the default placement project is ensured.
			projects, err = db.Console().Projects().GetOwnActive(ctx, withoutPlacement.ID)
			require.NoError(t, err)
			require.Len(t, projects, 1)
			require.Equal(t, storj.DefaultPlacement, projects[0].DefaultPlacement)
			requireLockedDown(t, projects[0])

			// with too few active nodes, the own placement project is locked down.
			projects, err = db.Console().Projects().GetOwnActive(ctx, fewNodes.ID)
			require.NoError(t, err)
			require.Len(t, projects, 2)
			for _, project := range projects {
				requireLockedDown(t, project)
				switch project.DefaultPlacement {
				case storj.DefaultPlacement:
					require.Equal(t, "Disabled due to not enough SNOs", project.Description)
				case ownPlacement + 1:
					require.Equal(t, "Disabled due to not enough nodes", project.Description)
				default:
					t.Fatalf("unexpected placement %d", project.DefaultPlacement)
				}
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

		// with a lower minimum the locked down own placement project is enabled.
		chore = planaccess.NewChore(zaptest.NewLogger(t), db.OverlayCache(), db.Console(), accountingDB,
			nodeselection.PlacementDefinitions{
				storj.DefaultPlacement: {ID: storj.DefaultPlacement, Name: "global"},
				ownPlacement + 1:       {ID: ownPlacement + 1, Name: string(nodes.EncodeOwner(fewNodes.ID))},
			},
			satelliteID,
			planaccess.Config{Interval: time.Hour, OnlineWindow: 4 * time.Hour, TallyLookback: 48 * time.Hour, MinNodes: 2},
		)
		require.NoError(t, chore.RunOnce(ctx))

		projects, err := db.Console().Projects().GetOwnActive(ctx, fewNodes.ID)
		require.NoError(t, err)
		require.Len(t, projects, 2)
		for _, project := range projects {
			if project.DefaultPlacement != ownPlacement+1 {
				continue
			}
			require.Equal(t, "Stored on own storagenodes", project.Description)
			requireInt64(t, 1_000_000_000, project.SegmentLimit)
			requireInt(t, 10000, project.RateLimitPut)
			requireInt(t, 10000, project.RateLimitGet)
		}
	})
}

func TestChoreExemptions(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		satelliteID := testrand.NodeID()
		now := time.Now()

		withoutNodes := addUser(ctx, t, db, "without-nodes@storj.test", console.Active)
		fewNodes := addUser(ctx, t, db, "few-nodes@storj.test", console.Active)
		inactive := addUser(ctx, t, db, "inactive@storj.test", console.Inactive)

		// a project with the name of a created project already exists.
		_, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:             fmt.Sprintf("placement %d", ownPlacement),
			OwnerID:          withoutNodes.ID,
			DefaultPlacement: ownPlacement + 5,
		})
		require.NoError(t, err)

		// too few nodes, but exempt, so the own placement is enabled.
		setOwner(ctx, t, db, satelliteID, fewNodes.ID, addNode(ctx, t, db, memory.GB, now), addNode(ctx, t, db, 2*memory.GB, now))

		var exemptions []planaccess.Exemption
		require.NoError(t, yaml.Unmarshal([]byte(fmt.Sprintf(`
  - user: %s
    placement: %d
  - user: %s
    placement: 99
  - user: %s
    placement: %d
    limits:
      storage: 10TB
      rate-limit-put: 5
  - user: %s
    placement: %d
  - user: %s
    placement: %d
  # the default placement, with the user ID in the owner tag form.
  - user: %s
    placement: 0
    limits:
      segment: 42
  # invalid entries are skipped: a typo in the placement key (which must not
  # be mistaken for the default placement), a duplicate and an invalid user.
  - user: %s
    placements: [%d]
  - user: %s
    placement: %d
    limits:
      storage: 1TB
  - user: not-a-uuid
    placement: %d
`, withoutNodes.ID, ownPlacement, withoutNodes.ID, withoutNodes.ID, ownPlacement+1, fewNodes.ID, ownPlacement, inactive.ID, ownPlacement,
			nodes.EncodeOwner(withoutNodes.ID), fewNodes.ID, ownPlacement+2, fewNodes.ID, ownPlacement, ownPlacement)), &exemptions))

		chore := planaccess.NewChore(zaptest.NewLogger(t), db.OverlayCache(), db.Console(), db.StoragenodeAccounting(),
			nodeselection.PlacementDefinitions{
				storj.DefaultPlacement: {ID: storj.DefaultPlacement, Name: "global"},
				ownPlacement:           {ID: ownPlacement, Name: "first"},
				ownPlacement + 1:       {ID: ownPlacement + 1, Name: "second"},
				ownPlacement + 2:       {ID: ownPlacement + 2, Name: string(nodes.EncodeOwner(fewNodes.ID))},
			},
			satelliteID,
			planaccess.Config{Interval: time.Hour, OnlineWindow: 4 * time.Hour, TallyLookback: 48 * time.Hour, MinNodes: 20, Exemptions: exemptions},
		)

		type expectedProject struct {
			description  string
			storage      int64
			bandwidth    *int64
			segment      int64
			rateLimitPut int
		}
		pb := int64(memory.PB)
		expected := map[uuid.UUID]map[storj.PlacementConstraint]expectedProject{
			// the undefined placement is skipped.
			withoutNodes.ID: {
				storj.DefaultPlacement: {description: "Provisioned to placement global", storage: pb, bandwidth: &pb, segment: 42, rateLimitPut: 10000},
				ownPlacement:           {description: "Provisioned to placement first", storage: pb, bandwidth: &pb, rateLimitPut: 10000},
				ownPlacement + 1:       {description: "Provisioned to placement second", storage: memory.TB.Int64() * 10, bandwidth: &pb, rateLimitPut: 5},
			},
			fewNodes.ID: {
				ownPlacement:     {description: "Provisioned to placement first", storage: pb, bandwidth: &pb, rateLimitPut: 10000},
				ownPlacement + 2: {description: "Stored on own storagenodes", storage: int64(float64(3*memory.GB) * 0.95), rateLimitPut: 10000},
			},
		}

		check := func() {
			for owner, placements := range expected {
				projects, err := db.Console().Projects().GetOwnActive(ctx, owner)
				require.NoError(t, err)

				names := map[string]bool{}
				found := map[storj.PlacementConstraint]bool{}
				for _, project := range projects {
					require.False(t, names[project.Name], "duplicated project name %q", project.Name)
					names[project.Name] = true

					want, ok := placements[project.DefaultPlacement]
					if !ok && project.DefaultPlacement == storj.DefaultPlacement {
						require.Equal(t, "Disabled due to not enough SNOs", project.Description)
						requireLockedDown(t, project)
						found[project.DefaultPlacement] = true
						continue
					}
					if !ok {
						continue
					}
					require.Equal(t, want.description, project.Description)
					require.NotNil(t, project.StorageLimit)
					require.Equal(t, want.storage, project.StorageLimit.Int64())
					if want.bandwidth != nil {
						require.NotNil(t, project.BandwidthLimit)
						require.Equal(t, *want.bandwidth, project.BandwidthLimit.Int64())
					}
					if want.segment == 0 {
						want.segment = 1_000_000_000
					}
					requireInt64(t, want.segment, project.SegmentLimit)
					requireInt(t, want.rateLimitPut, project.RateLimitPut)
					requireInt(t, 10000, project.RateLimitGet)
					found[project.DefaultPlacement] = true
				}
				_, defaultExempt := placements[storj.DefaultPlacement]
				if defaultExempt {
					require.Len(t, found, len(placements), "owner %s", owner)
				} else {
					require.Len(t, found, len(placements)+1, "owner %s", owner)
				}
			}

			// withoutNodes: the pre-existing project, the default and the two exempt ones.
			projects, err := db.Console().Projects().GetOwnActive(ctx, withoutNodes.ID)
			require.NoError(t, err)
			require.Len(t, projects, 4)

			// inactive users don't get projects, even if they are exempt.
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

func TestExemptionsYAML(t *testing.T) {
	user := testrand.UUID()

	var config struct {
		Exemptions []planaccess.Exemption `yaml:"plan-access.exemptions"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(fmt.Sprintf(`
plan-access.exemptions:
  - user: %s
    placement: 1
  - user: %s
    placement: 3
    limits:
      storage: 1TB
      segment: 7
`, user, user)), &config))

	storage, segment := memory.TB, int64(7)
	first, third := storj.PlacementConstraint(1), storj.PlacementConstraint(3)
	require.Equal(t, []planaccess.Exemption{
		{User: user.String(), Placement: &first},
		{User: user.String(), Placement: &third, Limits: planaccess.Limits{Storage: &storage, Segment: &segment}},
	}, config.Exemptions)

	// invalid entries don't fail the parsing, they are skipped by the chore.
	require.NoError(t, yaml.Unmarshal([]byte("plan-access.exemptions:\n  - user: invalid\n    placements: [1]\n"), &config))
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

func requireLockedDown(t *testing.T, project console.Project) {
	t.Helper()
	requireInt64(t, 0, project.SegmentLimit)
	requireInt(t, 0, project.RateLimitPut)
	requireInt(t, 0, project.RateLimitGet)
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
