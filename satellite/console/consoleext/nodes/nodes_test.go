// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/overlay"
)

// stubOverlayDB implements only the one method the extension uses. The embedded
// nil interface panics on anything else, which is what we want: the extension
// should not be reaching for the rest of overlay.DB.
type stubOverlayDB struct {
	overlay.DB

	nodes []*overlay.NodeDossier
	err   error

	gotEmail string
	gotLimit int
}

func (s *stubOverlayDB) GetNodesByEmailInsensitive(ctx context.Context, email string, limit int) ([]*overlay.NodeDossier, error) {
	s.gotEmail = email
	s.gotLimit = limit
	if s.err != nil {
		return nil, s.err
	}
	if len(s.nodes) > limit {
		return s.nodes[:limit], nil
	}
	return s.nodes, nil
}

func dossier(id storj.NodeID, lastContactSuccess time.Time) *overlay.NodeDossier {
	return &overlay.NodeDossier{
		Node: pb.Node{
			Id:      id,
			Address: &pb.NodeAddress{Address: "storj.test:28967"},
		},
		Operator: pb.NodeOperator{
			Email:          "Operator@Storj.Test",
			Wallet:         "0xabc",
			WalletFeatures: []string{"zksync"},
		},
		Capacity:   pb.NodeCapacity{FreeDisk: 1234},
		PieceCount: 42,
		Reputation: overlay.NodeStats{
			LastContactSuccess: lastContactSuccess,
		},
		Version:    pb.NodeVersion{Version: "v1.2.3"},
		LastIPPort: "1.2.3.4:28967",
		CreatedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func serve(t *testing.T, ext *Extension, user *console.User) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v0/nodes", nil)
	if user != nil {
		req = req.WithContext(console.WithUser(req.Context(), user))
	}

	rec := httptest.NewRecorder()
	ext.GetNodes(rec, req)
	return rec
}

func TestGetNodes(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		ext := New(zaptest.NewLogger(t), &stubOverlayDB{})
		rec := serve(t, ext, nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// Nodes are matched on an operator email that nobody verified, so an
	// unverified console account must not be able to resolve one.
	for _, status := range []console.UserStatus{
		console.Inactive,
		console.PendingBotVerification,
		console.LegalHold,
		console.PendingDeletion,
	} {
		t.Run("status "+status.String()+" is rejected", func(t *testing.T) {
			db := &stubOverlayDB{nodes: []*overlay.NodeDossier{dossier(testrand.NodeID(), now)}}
			ext := New(zaptest.NewLogger(t), db)

			rec := serve(t, ext, &console.User{Email: "operator@storj.test", Status: status})

			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Empty(t, db.gotEmail, "the database should not be queried at all")
		})
	}

	t.Run("active user gets their nodes", func(t *testing.T) {
		onlineID, offlineID := testrand.NodeID(), testrand.NodeID()
		db := &stubOverlayDB{nodes: []*overlay.NodeDossier{
			dossier(onlineID, now.Add(-time.Hour)),
			dossier(offlineID, now.Add(-24*time.Hour)),
		}}
		ext := New(zaptest.NewLogger(t), db)
		ext.nowFn = func() time.Time { return now }

		rec := serve(t, ext, &console.User{Email: "operator@storj.test", Status: console.Active})
		require.Equal(t, http.StatusOK, rec.Code)

		var page Page
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&page))

		require.Equal(t, "operator@storj.test", db.gotEmail)
		require.False(t, page.Truncated)
		require.Len(t, page.Nodes, 2)

		require.Equal(t, onlineID.String(), page.Nodes[0].ID)
		require.True(t, page.Nodes[0].Online, "contacted an hour ago")
		require.Equal(t, "1.2.3.4:28967", page.Nodes[0].LastIPPort)
		require.Equal(t, "0xabc", page.Nodes[0].Wallet)
		require.Equal(t, []string{"zksync"}, page.Nodes[0].WalletFeatures)
		require.EqualValues(t, 42, page.Nodes[0].PieceCount)
		require.EqualValues(t, 1234, page.Nodes[0].FreeDisk)
		require.Equal(t, "v1.2.3", page.Nodes[0].Version)

		require.False(t, page.Nodes[1].Online, "contacted a day ago")
	})

	t.Run("a node contacted exactly at the window edge is offline", func(t *testing.T) {
		db := &stubOverlayDB{nodes: []*overlay.NodeDossier{
			dossier(testrand.NodeID(), now.Add(-onlineWindow)),
		}}
		ext := New(zaptest.NewLogger(t), db)
		ext.nowFn = func() time.Time { return now }

		rec := serve(t, ext, &console.User{Email: "operator@storj.test", Status: console.Active})
		require.Equal(t, http.StatusOK, rec.Code)

		var page Page
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&page))
		require.Len(t, page.Nodes, 1)
		require.False(t, page.Nodes[0].Online)
	})

	t.Run("disqualification reason is rendered", func(t *testing.T) {
		dq := now.Add(-time.Hour)
		reason := overlay.DisqualificationReasonAuditFailure
		d := dossier(testrand.NodeID(), now)
		d.Disqualified = &dq
		d.DisqualificationReason = &reason

		ext := New(zaptest.NewLogger(t), &stubOverlayDB{nodes: []*overlay.NodeDossier{d}})
		ext.nowFn = func() time.Time { return now }

		rec := serve(t, ext, &console.User{Email: "operator@storj.test", Status: console.Active})
		require.Equal(t, http.StatusOK, rec.Code)

		var page Page
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&page))
		require.Len(t, page.Nodes, 1)
		require.NotNil(t, page.Nodes[0].Disqualified)
		require.NotNil(t, page.Nodes[0].DisqualificationReason)
		require.Equal(t, "Audit Failure", *page.Nodes[0].DisqualificationReason)
	})

	t.Run("an operator over the cap gets a truncated list", func(t *testing.T) {
		all := make([]*overlay.NodeDossier, maxNodes+1)
		for i := range all {
			all[i] = dossier(testrand.NodeID(), now)
		}

		db := &stubOverlayDB{nodes: all}
		ext := New(zaptest.NewLogger(t), db)
		ext.nowFn = func() time.Time { return now }

		rec := serve(t, ext, &console.User{Email: "operator@storj.test", Status: console.Active})
		require.Equal(t, http.StatusOK, rec.Code)

		var page Page
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&page))

		require.Equal(t, maxNodes+1, db.gotLimit, "should ask for one over the cap to detect truncation")
		require.Len(t, page.Nodes, maxNodes)
		require.True(t, page.Truncated)
	})

	t.Run("an operator exactly at the cap is not marked truncated", func(t *testing.T) {
		all := make([]*overlay.NodeDossier, maxNodes)
		for i := range all {
			all[i] = dossier(testrand.NodeID(), now)
		}

		ext := New(zaptest.NewLogger(t), &stubOverlayDB{nodes: all})
		ext.nowFn = func() time.Time { return now }

		rec := serve(t, ext, &console.User{Email: "operator@storj.test", Status: console.Active})
		require.Equal(t, http.StatusOK, rec.Code)

		var page Page
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&page))
		require.Len(t, page.Nodes, maxNodes)
		require.False(t, page.Truncated)
	})

	t.Run("an operator with no nodes gets an empty list, not null", func(t *testing.T) {
		ext := New(zaptest.NewLogger(t), &stubOverlayDB{})
		rec := serve(t, ext, &console.User{Email: "nobody@storj.test", Status: console.Active})
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"nodes":[]`)
	})
}
