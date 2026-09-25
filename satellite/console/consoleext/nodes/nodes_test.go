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

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleext"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

// stubOverlayDB implements only the few methods the extension uses. The embedded
// nil interface panics on anything else, which is what we want: the extension
// should not be reaching for the rest of overlay.DB.
type stubOverlayDB struct {
	overlay.DB

	nodes []*overlay.NodeDossier
	err   error

	gotEmail string
	gotLimit int
	gotTags  nodeselection.NodeTags
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

func (s *stubOverlayDB) Get(ctx context.Context, nodeID storj.NodeID) (*overlay.NodeDossier, error) {
	if s.err != nil {
		return nil, s.err
	}
	for _, n := range s.nodes {
		if n.Id == nodeID {
			return n, nil
		}
	}
	return nil, overlay.ErrNodeNotFound.New("%v", nodeID)
}

func (s *stubOverlayDB) UpdateNodeTags(ctx context.Context, tags nodeselection.NodeTags) error {
	if s.err != nil {
		return s.err
	}
	s.gotTags = append(s.gotTags, tags...)
	return nil
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

// satelliteID stands in for the identity this satellite signs owner tags with.
var satelliteID = testrand.NodeID()

func newExt(t *testing.T, db overlay.DB) *Extension {
	t.Helper()
	return New(zaptest.NewLogger(t), db, satelliteID)
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

// serveConfirm drives ConfirmNode through a real router, so that the {id} path
// variable is resolved the same way it is in production.
func serveConfirm(t *testing.T, ext *Extension, user *console.User, nodeID string) *httptest.ResponseRecorder {
	t.Helper()

	router := mux.NewRouter()
	ext.Register(router, consoleext.Deps{WithAuth: func(next http.Handler) http.Handler { return next }})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v0/nodes/"+nodeID+"/confirm", nil)
	if user != nil {
		req = req.WithContext(console.WithUser(req.Context(), user))
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestGetNodes(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		ext := newExt(t, &stubOverlayDB{})
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
			ext := newExt(t, db)

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
		ext := newExt(t, db)
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
		ext := newExt(t, db)
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

		ext := newExt(t, &stubOverlayDB{nodes: []*overlay.NodeDossier{d}})
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
		ext := newExt(t, db)
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

		ext := newExt(t, &stubOverlayDB{nodes: all})
		ext.nowFn = func() time.Time { return now }

		rec := serve(t, ext, &console.User{Email: "operator@storj.test", Status: console.Active})
		require.Equal(t, http.StatusOK, rec.Code)

		var page Page
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&page))
		require.Len(t, page.Nodes, maxNodes)
		require.False(t, page.Truncated)
	})

	t.Run("an operator with no nodes gets an empty list, not null", func(t *testing.T) {
		ext := newExt(t, &stubOverlayDB{})
		rec := serve(t, ext, &console.User{Email: "nobody@storj.test", Status: console.Active})
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"nodes":[]`)
	})

	t.Run("ownership is reported from the owner tag", func(t *testing.T) {
		user := activeUser(t)
		otherUserID := testrand.UUID()

		mine := dossier(testrand.NodeID(), now)
		mine.Tags = ownerTags(mine.Id, satelliteID, user.ID)

		// the same tag, but naming somebody else: the node changed hands, so
		// this user has not confirmed it.
		theirs := dossier(testrand.NodeID(), now)
		theirs.Tags = ownerTags(theirs.Id, satelliteID, otherUserID)

		// a tag naming this user, but signed by a node rather than by us.
		forged := dossier(testrand.NodeID(), now)
		forged.Tags = ownerTags(forged.Id, testrand.NodeID(), user.ID)

		untagged := dossier(testrand.NodeID(), now)

		ext := newExt(t, &stubOverlayDB{nodes: []*overlay.NodeDossier{mine, theirs, forged, untagged}})
		ext.nowFn = func() time.Time { return now }

		rec := serve(t, ext, user)
		require.Equal(t, http.StatusOK, rec.Code)

		var page Page
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&page))
		require.Len(t, page.Nodes, 4)

		require.True(t, page.Nodes[0].Confirmed, "tagged by this satellite for this user")
		require.False(t, page.Nodes[1].Confirmed, "tagged for a different user")
		require.False(t, page.Nodes[2].Confirmed, "not signed by this satellite")
		require.False(t, page.Nodes[3].Confirmed, "not tagged at all")
	})
}

func TestConfirmNode(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		db := &stubOverlayDB{nodes: []*overlay.NodeDossier{dossier(testrand.NodeID(), now)}}
		rec := serveConfirm(t, newExt(t, db), nil, db.nodes[0].Id.String())

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.Empty(t, db.gotTags)
	})

	t.Run("unverified account is rejected", func(t *testing.T) {
		db := &stubOverlayDB{nodes: []*overlay.NodeDossier{dossier(testrand.NodeID(), now)}}
		user := activeUser(t)
		user.Status = console.Inactive

		rec := serveConfirm(t, newExt(t, db), user, db.nodes[0].Id.String())

		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Empty(t, db.gotTags)
	})

	t.Run("a malformed node ID is rejected", func(t *testing.T) {
		db := &stubOverlayDB{}
		rec := serveConfirm(t, newExt(t, db), activeUser(t), "not-a-node-id")

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Empty(t, db.gotTags)
	})

	t.Run("an unknown node is not found", func(t *testing.T) {
		db := &stubOverlayDB{}
		rec := serveConfirm(t, newExt(t, db), activeUser(t), testrand.NodeID().String())

		require.Equal(t, http.StatusNotFound, rec.Code)
		require.Empty(t, db.gotTags)
	})

	// The listing is the only thing that ties a user to a node, so confirming
	// has to re-run the same check rather than trust the caller.
	t.Run("a node registered to somebody else is rejected", func(t *testing.T) {
		d := dossier(testrand.NodeID(), now)
		d.Operator.Email = "somebody@else.test"
		db := &stubOverlayDB{nodes: []*overlay.NodeDossier{d}}

		rec := serveConfirm(t, newExt(t, db), activeUser(t), d.Id.String())

		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Empty(t, db.gotTags)
	})

	t.Run("owner tag is written with the satellite as signer", func(t *testing.T) {
		d := dossier(testrand.NodeID(), now)
		db := &stubOverlayDB{nodes: []*overlay.NodeDossier{d}}
		ext := newExt(t, db)
		ext.nowFn = func() time.Time { return now }

		// the node reports "Operator@Storj.Test"; the account is lowercase.
		user := activeUser(t)
		rec := serveConfirm(t, ext, user, d.Id.String())

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"confirmed":true`)

		require.Len(t, db.gotTags, 1)
		require.Equal(t, nodeselection.NodeTag{
			NodeID:   d.Id,
			Name:     OwnerTagName,
			Value:    EncodeOwner(user.ID),
			SignedAt: now,
			Signer:   satelliteID,
		}, db.gotTags[0])
	})
}

func activeUser(t *testing.T) *console.User {
	t.Helper()
	return &console.User{ID: testrand.UUID(), Email: "operator@storj.test", Status: console.Active}
}

func ownerTags(nodeID, signer storj.NodeID, owner uuid.UUID) nodeselection.NodeTags {
	return nodeselection.NodeTags{{
		NodeID:   nodeID,
		Name:     OwnerTagName,
		Value:    EncodeOwner(owner),
		SignedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Signer:   signer,
	}}
}

func TestOwnerTagEncoding(t *testing.T) {
	owner, err := uuid.FromString("0123abcd-4567-89ef-0123-456789abcdef")
	require.NoError(t, err)

	value := EncodeOwner(owner)
	require.Equal(t, "0123abcd456789ef0123456789abcdef", string(value))

	decoded, err := DecodeOwner(value)
	require.NoError(t, err)
	require.Equal(t, owner, decoded)

	t.Run("readable by nodeselection", func(t *testing.T) {
		satelliteID := testrand.NodeID()
		node := nodeselection.SelectedNode{
			ID:   testrand.NodeID(),
			Tags: ownerTags(testrand.NodeID(), satelliteID, owner),
		}
		attr := nodeselection.NodeTagAttribute(satelliteID, OwnerTagName)
		require.Equal(t, "0123abcd456789ef0123456789abcdef", attr(node))
	})

	for name, invalid := range map[string][]byte{
		"empty":      nil,
		"raw bytes":  owner.Bytes(),
		"upper case": []byte("0123ABCD456789EF0123456789ABCDEF"),
		"dashes":     []byte(owner.String()),
		"not hex":    []byte("0123abcd456789ef0123456789abcdeg"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeOwner(invalid)
			require.Error(t, err)
		})
	}
}
