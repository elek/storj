// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package telegram

import (
	"context"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleext/nodes"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

// maxNodes caps how many nodes of an operator are scanned for ownership.
const maxNodes = 1000

// Links stores which Telegram chat belongs to which console user.
//
// The chats are kept in their own table, keyed by the user. The nodes of a
// user are found by the owner tag of consoleext/nodes, so a node which has
// changed owners is reported to the chat of its current owner only.
type Links struct {
	overlayDB   overlay.DB
	users       console.Users
	chats       Persistence
	satelliteID storj.NodeID
}

// NewLinks creates the Telegram link store.
func NewLinks(overlayDB overlay.DB, consoleDB console.DB, chats Persistence, satelliteID storj.NodeID) *Links {
	return &Links{
		overlayDB:   overlayDB,
		users:       consoleDB.Users(),
		chats:       chats,
		satelliteID: satelliteID,
	}
}

// ErrNoOwnedNodes is returned when a user without confirmed nodes connects a chat.
var ErrNoOwnedNodes = Error.New("no confirmed nodes")

// Link records chatID as the Telegram chat of the user. It returns the number
// of nodes the user has confirmed, which are the ones notified in the chat.
func (l *Links) Link(ctx context.Context, userID uuid.UUID, chatID int64) (count int, err error) {
	defer mon.Task()(&ctx)(&err)

	dossiers, err := l.nodesOf(ctx, userID)
	if err != nil {
		return 0, err
	}

	for _, d := range dossiers {
		if l.ownerOf(d.Tags) == userID {
			count++
		}
	}
	if count == 0 {
		return 0, ErrNoOwnedNodes
	}

	return count, Error.Wrap(l.chats.SaveChatID(ctx, userID, chatID))
}

// Unlink disconnects the Telegram chat of the user.
func (l *Links) Unlink(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	return Error.Wrap(l.chats.DeleteChatID(ctx, userID))
}

// ChatOf returns the Telegram chat of the user. ok is false when the user has
// not connected one.
func (l *Links) ChatOf(ctx context.Context, userID uuid.UUID) (chatID int64, ok bool, err error) {
	defer mon.Task()(&ctx)(&err)

	chatID, err = l.chats.GetChatID(ctx, userID)
	if err != nil {
		return 0, false, Error.Wrap(err)
	}
	// chat ID 0 is never a valid Telegram chat.
	return chatID, chatID != 0, nil
}

// OwnerOf returns the confirmed owner of the node.
func (l *Links) OwnerOf(ctx context.Context, nodeID storj.NodeID) (_ uuid.UUID, err error) {
	defer mon.Task()(&ctx)(&err)

	tags, err := l.overlayDB.GetNodeTags(ctx, nodeID)
	if err != nil {
		return uuid.UUID{}, Error.Wrap(err)
	}
	return l.ownerOf(tags), nil
}

// nodesOf returns the nodes registered with the user's email address. That is
// where the owner tags of the user are.
func (l *Links) nodesOf(ctx context.Context, userID uuid.UUID) ([]*overlay.NodeDossier, error) {
	user, err := l.users.Get(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	dossiers, err := l.overlayDB.GetNodesByEmailInsensitive(ctx, user.Email, maxNodes)
	return dossiers, Error.Wrap(err)
}

func (l *Links) ownerOf(tags nodeselection.NodeTags) uuid.UUID {
	tag, err := tags.FindBySignerAndName(l.satelliteID, nodes.OwnerTagName)
	if err != nil {
		return uuid.UUID{}
	}
	owner, err := nodes.DecodeOwner(tag.Value)
	if err != nil {
		return uuid.UUID{}
	}
	return owner
}
