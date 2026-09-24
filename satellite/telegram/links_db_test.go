// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package telegram_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleext/nodes"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/dbaccess"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/satellite/telegram"
)

// TestDB checks the chats against the real telegram_chats table.
func TestDB(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		access, ok := db.(dbaccess.Access)
		require.True(t, ok)

		chats, err := telegram.NewDB(ctx, zaptest.NewLogger(t), telegram.NewMigration(access), access)
		require.NoError(t, err)

		// the migration is idempotent, and independent of the satellitedb one
		_, err = telegram.NewDB(ctx, zaptest.NewLogger(t), telegram.NewMigration(access), access)
		require.NoError(t, err)
		require.NoError(t, db.CheckVersion(ctx))

		user, other := testrand.UUID(), testrand.UUID()

		chatID, err := chats.GetChatID(ctx, user)
		require.NoError(t, err)
		require.Zero(t, chatID)

		createdAt := func() (createdAt time.Time) {
			err := access.GetDB().QueryRowContext(ctx, `SELECT created_at FROM telegram_chats WHERE user_id = $1`, user).Scan(&createdAt)
			require.NoError(t, err)
			return createdAt
		}

		require.NoError(t, chats.SaveChatID(ctx, user, 1001))
		require.NoError(t, chats.SaveChatID(ctx, other, 3003))
		connected := createdAt()
		require.WithinDuration(t, time.Now(), connected, time.Minute)

		time.Sleep(10 * time.Millisecond)

		// chat IDs of supergroups don't fit into 32 bits
		require.NoError(t, chats.SaveChatID(ctx, user, -1002003004005))
		chatID, err = chats.GetChatID(ctx, user)
		require.NoError(t, err)
		require.EqualValues(t, -1002003004005, chatID)
		require.True(t, createdAt().After(connected), "replacing the chat refreshes created_at")

		require.NoError(t, chats.DeleteChatID(ctx, user))
		chatID, err = chats.GetChatID(ctx, user)
		require.NoError(t, err)
		require.Zero(t, chatID)

		// deleting a missing chat is not an error
		require.NoError(t, chats.DeleteChatID(ctx, user))

		chatID, err = chats.GetChatID(ctx, other)
		require.NoError(t, err)
		require.EqualValues(t, 3003, chatID)
	})
}

// TestLinksDB checks the links against the real database: that the owner tags
// of the nodes are respected, and that writing a link again replaces it.
func TestLinksDB(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		satelliteID := testrand.NodeID()
		cache := db.OverlayCache()

		user, err := db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "Operator",
			Email:        "operator@storj.test",
			PasswordHash: []byte("hash"),
		})
		require.NoError(t, err)

		var nodeIDs []storj.NodeID
		for range 3 {
			nodeID := testrand.NodeID()
			nodeIDs = append(nodeIDs, nodeID)
			require.NoError(t, cache.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
				NodeID:   nodeID,
				IsUp:     true,
				Address:  &pb.NodeAddress{Address: "1.2.3.4"},
				Version:  &pb.NodeVersion{Version: "v0.0.0"},
				Operator: &pb.NodeOperator{Email: "Operator@Storj.Test"},
			}, time.Now(), overlay.NodeSelectionConfig{}))
		}

		// the first two are confirmed, the way consoleext/nodes records it
		for _, nodeID := range nodeIDs[:2] {
			require.NoError(t, cache.UpdateNodeTags(ctx, nodeselection.NodeTags{{
				NodeID:   nodeID,
				Name:     nodes.OwnerTagName,
				Value:    nodes.EncodeOwner(user.ID),
				SignedAt: time.Now(),
				Signer:   satelliteID,
			}}))
		}

		access, ok := db.(dbaccess.Access)
		require.True(t, ok)
		chats, err := telegram.NewDB(ctx, zaptest.NewLogger(t), telegram.NewMigration(access), access)
		require.NoError(t, err)

		links := telegram.NewLinks(cache, db.Console(), chats, satelliteID)

		count, err := links.Link(ctx, user.ID, 1001)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		count, err = links.Link(ctx, user.ID, -2002)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		tags, err := cache.GetNodeTags(ctx, nodeIDs[0])
		require.NoError(t, err)
		require.Len(t, tags, 1, "only the owner tag, the link is not stored in node tags")

		chatID, ok, err := links.ChatOf(ctx, user.ID)
		require.NoError(t, err)
		require.True(t, ok)
		require.EqualValues(t, -2002, chatID)

		owner, err := links.OwnerOf(ctx, nodeIDs[1])
		require.NoError(t, err)
		require.Equal(t, user.ID, owner)

		owner, err = links.OwnerOf(ctx, nodeIDs[2])
		require.NoError(t, err)
		require.True(t, owner.IsZero(), "an unconfirmed node has no owner")

		require.NoError(t, links.Unlink(ctx, user.ID))
		_, ok, err = links.ChatOf(ctx, user.ID)
		require.NoError(t, err)
		require.False(t, ok)
	})
}
