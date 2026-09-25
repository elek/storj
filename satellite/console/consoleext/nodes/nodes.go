// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package nodes exposes the storage nodes operated by the logged in console
// user, matched on the operator email the node reports at check-in, and lets
// that user confirm ownership of them.
package nodes

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleext"
	"storj.io/storj/satellite/nodeselection"
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

// OwnerTagName is the node tag under which the confirmed owner is recorded. Its
// value is the console user's ID as lower case hex text (see EncodeOwner), so
// that the nodeselection package, which reads tag values as strings, can match
// on it in placement and selector definitions.
//
// The tag is only meaningful together with its signer: node_tags rows are keyed
// by (node_id, name, signer), so an owner tag written by this satellite can only
// ever be replaced by this satellite. A node pushing its own tag sets at
// check-in cannot produce a row with the satellite's signer unless the
// satellite actually signed it (satellite/contact/service.go:194).
const OwnerTagName = "owner"

// EncodeOwner returns the owner tag value for the given user: the UTF-8 text of
// the user ID's 16 bytes in lower case hex, without dashes.
func EncodeOwner(owner uuid.UUID) []byte {
	return []byte(hex.EncodeToString(owner.Bytes()))
}

// DecodeOwner parses an owner tag value written by EncodeOwner. Only the exact
// form EncodeOwner produces is accepted, so that string comparisons of tag
// values (as nodeselection does) agree with comparisons of the decoded IDs.
func DecodeOwner(value []byte) (uuid.UUID, error) {
	var owner uuid.UUID
	if hex.EncodedLen(len(owner)) != len(value) {
		return uuid.UUID{}, Error.New("invalid owner tag length: %d", len(value))
	}
	if _, err := hex.Decode(owner[:], value); err != nil {
		return uuid.UUID{}, Error.Wrap(err)
	}
	if string(EncodeOwner(owner)) != string(value) {
		return uuid.UUID{}, Error.New("owner tag is not lower case hex")
	}
	return owner, nil
}

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
	// Confirmed is true when this satellite has recorded an owner tag naming
	// the requesting user for this node.
	Confirmed bool `json:"confirmed"`
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
	// satelliteID is this satellite's own node ID, used as the signer of the
	// owner tags it writes.
	satelliteID storj.NodeID
	nowFn       func() time.Time
}

// New creates the nodes console extension.
func New(log *zap.Logger, overlayDB overlay.DB, satelliteID storj.NodeID) *Extension {
	return &Extension{
		log:         log,
		overlayDB:   overlayDB,
		satelliteID: satelliteID,
		nowFn:       time.Now,
	}
}

// Name implements consoleext.Extension.
func (e *Extension) Name() string { return "nodes" }

// Register implements consoleext.Extension.
func (e *Extension) Register(router *mux.Router, deps consoleext.Deps) {
	nodesRouter := router.PathPrefix("/api/v0/nodes").Subrouter()
	nodesRouter.Use(deps.WithAuth)
	nodesRouter.Handle("", http.HandlerFunc(e.GetNodes)).Methods(http.MethodGet, http.MethodOptions)
	nodesRouter.Handle("/{id}/confirm", http.HandlerFunc(e.ConfirmNode)).Methods(http.MethodPost, http.MethodOptions)
}

// GetNodes returns the nodes whose operator email matches the user's email.
func (e *Extension) GetNodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	user, status, err := e.activeUser(ctx)
	if err != nil {
		e.serveJSONError(ctx, w, status, err)
		return
	}

	page, err := e.getNodes(ctx, user)
	if err != nil {
		e.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(page)
	if err != nil {
		e.log.Error("failed to write json nodes response", zap.Error(Error.Wrap(err)))
	}
}

// ConfirmNode records that the requesting user owns the node named in the path,
// as an owner tag signed by this satellite's identity.
func (e *Extension) ConfirmNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	user, status, err := e.activeUser(ctx)
	if err != nil {
		e.serveJSONError(ctx, w, status, err)
		return
	}

	nodeID, err := storj.NodeIDFromString(mux.Vars(r)["id"])
	if err != nil {
		e.serveJSONError(ctx, w, http.StatusBadRequest, Error.New("invalid node ID"))
		return
	}

	dossier, err := e.overlayDB.Get(ctx, nodeID)
	if err != nil {
		if overlay.ErrNodeNotFound.Has(err) {
			e.serveJSONError(ctx, w, http.StatusNotFound, Error.New("node not found"))
			return
		}
		e.serveJSONError(ctx, w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	// The same match the listing uses. Confirming re-checks it rather than
	// trusting that the node was on a list the user was shown earlier.
	if !strings.EqualFold(dossier.Operator.Email, user.Email) {
		e.serveJSONError(ctx, w, http.StatusForbidden, Error.New("node is not registered to your email address"))
		return
	}

	err = e.overlayDB.UpdateNodeTags(ctx, nodeselection.NodeTags{{
		NodeID:   nodeID,
		Name:     OwnerTagName,
		Value:    EncodeOwner(user.ID),
		SignedAt: e.nowFn(),
		Signer:   e.satelliteID,
	}})
	if err != nil {
		e.serveJSONError(ctx, w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	err = json.NewEncoder(w).Encode(struct {
		Confirmed bool `json:"confirmed"`
	}{Confirmed: true})
	if err != nil {
		e.log.Error("failed to write json confirm response", zap.Error(Error.Wrap(err)))
	}
}

// activeUser returns the logged in user along with the HTTP status to report if
// they may not resolve nodes.
func (e *Extension) activeUser(ctx context.Context) (*console.User, int, error) {
	user, err := console.GetUser(ctx)
	if err != nil {
		return nil, http.StatusUnauthorized, err
	}

	// Nodes are matched on a self-reported operator email, so we only resolve
	// them for users who have proven they own the address they registered with.
	if user.Status != console.Active {
		return nil, http.StatusForbidden, Error.New("email address is not verified")
	}

	return user, http.StatusOK, nil
}

func (e *Extension) getNodes(ctx context.Context, user *console.User) (_ Page, err error) {
	defer mon.Task()(&ctx)(&err)

	// Ask for one more than the cap so we can tell a full page from a truncated one.
	dossiers, err := e.overlayDB.GetNodesByEmailInsensitive(ctx, user.Email, maxNodes+1)
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
			Confirmed:              e.confirmedBy(d.Tags, user.ID),
		})
	}

	return page, nil
}

// confirmedBy reports whether this satellite has tagged the node as owned by
// owner. A tag naming somebody else counts as unconfirmed, so that an operator
// who took over a node's email can claim it.
func (e *Extension) confirmedBy(tags nodeselection.NodeTags, owner uuid.UUID) bool {
	tag, err := tags.FindBySignerAndName(e.satelliteID, OwnerTagName)
	if err != nil {
		return false
	}
	recorded, err := DecodeOwner(tag.Value)
	if err != nil {
		return false
	}
	return recorded == owner
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
