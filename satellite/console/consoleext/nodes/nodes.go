// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package nodes exposes the storage nodes operated by the logged in console
// user, matched on the operator email the node reports at check-in.
package nodes

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleext"
	"storj.io/storj/satellite/overlay"
)

var (
	mon = monkit.Package()

	// Error is the error class for the console nodes extension.
	Error = errs.Class("console nodes")
)

// maxNodes caps how many nodes a single operator can list. Operators with more
// nodes than this see a truncated list; the UI is told so it can say as much.
const maxNodes = 1000

// onlineWindow is how long after the last successful contact a node still
// counts as online. Matches satellite/admin/nodes.go.
const onlineWindow = 4 * time.Hour

// Node is a storage node as shown in the console.
type Node struct {
	ID                     string     `json:"id"`
	Address                string     `json:"address"`
	LastIPPort             string     `json:"lastIpPort"`
	Wallet                 string     `json:"wallet"`
	WalletFeatures         []string   `json:"walletFeatures"`
	PieceCount             int64      `json:"pieceCount"`
	FreeDisk               int64      `json:"freeDisk"`
	Online                 bool       `json:"online"`
	LastContactSuccess     time.Time  `json:"lastContactSuccess"`
	LastContactFailure     time.Time  `json:"lastContactFailure"`
	VettedAt               *time.Time `json:"vettedAt"`
	Disqualified           *time.Time `json:"disqualified"`
	DisqualificationReason *string    `json:"disqualificationReason"`
	ExitFinishedAt         *time.Time `json:"exitFinishedAt"`
	CountryCode            string     `json:"countryCode"`
	Version                string     `json:"version"`
	CreatedAt              time.Time  `json:"createdAt"`
}

// Page is the response of the nodes endpoint.
type Page struct {
	Nodes []Node `json:"nodes"`
	// Truncated is true when the operator has more nodes than maxNodes.
	Truncated bool `json:"truncated"`
}

// Extension serves the list of nodes belonging to the logged in user.
type Extension struct {
	log       *zap.Logger
	overlayDB overlay.DB
	nowFn     func() time.Time
}

// New creates the nodes console extension.
func New(log *zap.Logger, overlayDB overlay.DB) *Extension {
	return &Extension{
		log:       log,
		overlayDB: overlayDB,
		nowFn:     time.Now,
	}
}

// Name implements consoleext.Extension.
func (e *Extension) Name() string { return "nodes" }

// Register implements consoleext.Extension.
func (e *Extension) Register(router *mux.Router, deps consoleext.Deps) {
	nodesRouter := router.PathPrefix("/api/v0/nodes").Subrouter()
	nodesRouter.Use(deps.WithAuth)
	nodesRouter.Handle("", http.HandlerFunc(e.GetNodes)).Methods(http.MethodGet, http.MethodOptions)
}

// GetNodes returns the nodes whose operator email matches the user's email.
func (e *Extension) GetNodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	user, err := console.GetUser(ctx)
	if err != nil {
		e.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		return
	}

	// Nodes are matched on a self-reported operator email, so we only resolve
	// them for users who have proven they own the address they registered with.
	if user.Status != console.Active {
		e.serveJSONError(ctx, w, http.StatusForbidden, Error.New("email address is not verified"))
		return
	}

	page, err := e.getNodes(ctx, user.Email)
	if err != nil {
		e.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(page)
	if err != nil {
		e.log.Error("failed to write json nodes response", zap.Error(Error.Wrap(err)))
	}
}

func (e *Extension) getNodes(ctx context.Context, email string) (_ Page, err error) {
	defer mon.Task()(&ctx)(&err)

	// Ask for one more than the cap so we can tell a full page from a truncated one.
	dossiers, err := e.overlayDB.GetNodesByEmailInsensitive(ctx, email, maxNodes+1)
	if err != nil {
		return Page{}, Error.Wrap(err)
	}

	page := Page{Truncated: len(dossiers) > maxNodes}
	if page.Truncated {
		dossiers = dossiers[:maxNodes]
	}

	now := e.nowFn()
	page.Nodes = make([]Node, 0, len(dossiers))
	for _, d := range dossiers {
		var dqReason *string
		if d.DisqualificationReason != nil {
			reason := disqualificationReasonToString(*d.DisqualificationReason)
			dqReason = &reason
		}

		page.Nodes = append(page.Nodes, Node{
			ID:                     d.Id.String(),
			Address:                d.Address.Address,
			LastIPPort:             d.LastIPPort,
			Wallet:                 d.Operator.Wallet,
			WalletFeatures:         d.Operator.WalletFeatures,
			PieceCount:             d.PieceCount,
			FreeDisk:               d.Capacity.FreeDisk,
			Online:                 now.Sub(d.Reputation.LastContactSuccess) < onlineWindow,
			LastContactSuccess:     d.Reputation.LastContactSuccess,
			LastContactFailure:     d.Reputation.LastContactFailure,
			VettedAt:               d.Reputation.Status.VettedAt,
			Disqualified:           d.Disqualified,
			DisqualificationReason: dqReason,
			ExitFinishedAt:         d.ExitStatus.ExitFinishedAt,
			CountryCode:            d.CountryCode.String(),
			Version:                d.Version.Version,
			CreatedAt:              d.CreatedAt,
		})
	}

	return page, nil
}

func (e *Extension) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, e.log, w, status, err)
}

func disqualificationReasonToString(reason overlay.DisqualificationReason) string {
	switch reason {
	case overlay.DisqualificationReasonAuditFailure:
		return "Audit Failure"
	case overlay.DisqualificationReasonSuspension:
		return "Suspension"
	case overlay.DisqualificationReasonNodeOffline:
		return "Node Offline"
	default:
		return "Unknown"
	}
}
