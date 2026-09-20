// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodes_test

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
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleext"
	"storj.io/storj/satellite/console/consoleext/nodes"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// TestConfirmThenList reproduces what the browser does: confirm a node, then
// reload the list. Both halves go through a real database.
func TestConfirmThenList(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()
		satelliteID := testrand.NodeID()

		nodeID := testrand.NodeID()
		require.NoError(t, cache.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
			NodeID:  nodeID,
			IsUp:    true,
			Address: &pb.NodeAddress{Address: "1.2.3.4"},
			Version: &pb.NodeVersion{Version: "v0.0.0"},
			Operator: &pb.NodeOperator{
				Email:  "Operator@Storj.Test",
				Wallet: "0x1234567890123456789012345678901234567890",
			},
		}, time.Now(), overlay.NodeSelectionConfig{OnlineWindow: 4 * time.Hour}))

		ext := nodes.New(zaptest.NewLogger(t), cache, satelliteID)
		user := &console.User{ID: testrand.UUID(), Email: "operator@storj.test", Status: console.Active}

		// mirrors consoleweb's withAuth, which hands the handler a *clone* of the
		// request carrying the authenticated user.
		withAuth := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.Clone(console.WithUser(r.Context(), user)))
			})
		}

		router := mux.NewRouter()
		ext.Register(router, consoleext.Deps{WithAuth: withAuth})

		do := func(method, path string) *httptest.ResponseRecorder {
			req := httptest.NewRequestWithContext(context.Background(), method, path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			return rec
		}

		rec := do(http.MethodPost, "/api/v0/nodes/"+nodeID.String()+"/confirm")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		// nodes check in constantly; the confirmation has to survive that.
		require.NoError(t, cache.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
			NodeID:  nodeID,
			IsUp:    true,
			Address: &pb.NodeAddress{Address: "1.2.3.4"},
			Version: &pb.NodeVersion{Version: "v0.0.0"},
			Operator: &pb.NodeOperator{
				Email:  "Operator@Storj.Test",
				Wallet: "0x1234567890123456789012345678901234567890",
			},
		}, time.Now(), overlay.NodeSelectionConfig{OnlineWindow: 4 * time.Hour}))

		// what the browser sees after a reload.
		rec = do(http.MethodGet, "/api/v0/nodes")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var page nodes.Page
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&page))
		require.Len(t, page.Nodes, 1)
		require.True(t, page.Nodes[0].Confirmed, "node should still be confirmed after a reload")
	})
}
