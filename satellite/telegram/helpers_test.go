// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package telegram

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleext/nodes"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

// memoryOverlay keeps nodes and node tags in memory, with the upsert semantics
// of the real node_tags table: one row per (node, name, signer). The embedded
// nil interface panics on anything else.
type memoryOverlay struct {
	overlay.DB

	mu    sync.Mutex
	nodes []*overlay.NodeDossier
	tags  map[storj.NodeID]nodeselection.NodeTags
}

func newMemoryOverlay() *memoryOverlay {
	return &memoryOverlay{tags: map[storj.NodeID]nodeselection.NodeTags{}}
}

func (m *memoryOverlay) addNode(email string) storj.NodeID {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := testrand.NodeID()
	m.nodes = append(m.nodes, &overlay.NodeDossier{
		Node:     pb.Node{Id: id},
		Operator: pb.NodeOperator{Email: email},
	})
	return id
}

func (m *memoryOverlay) GetNodesByEmailInsensitive(ctx context.Context, email string, limit int) ([]*overlay.NodeDossier, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*overlay.NodeDossier
	for _, n := range m.nodes {
		if strings.EqualFold(n.Operator.Email, email) && len(result) < limit {
			d := *n
			d.Tags = append(nodeselection.NodeTags(nil), m.tags[n.Id]...)
			result = append(result, &d)
		}
	}
	return result, nil
}

func (m *memoryOverlay) GetNodeTags(ctx context.Context, id storj.NodeID) (nodeselection.NodeTags, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append(nodeselection.NodeTags(nil), m.tags[id]...), nil
}

func (m *memoryOverlay) UpdateNodeTags(ctx context.Context, tags nodeselection.NodeTags) error {
	m.mu.Lock()
	defer m.mu.Unlock()
next:
	for _, tag := range tags {
		existing := m.tags[tag.NodeID]
		for i := range existing {
			if existing[i].Name == tag.Name && existing[i].Signer == tag.Signer {
				existing[i] = tag
				continue next
			}
		}
		m.tags[tag.NodeID] = append(existing, tag)
	}
	return nil
}

// confirm records user as the owner of the node, the way consoleext/nodes does.
func (m *memoryOverlay) confirm(nodeID storj.NodeID, userID uuid.UUID) {
	_ = m.UpdateNodeTags(context.Background(), nodeselection.NodeTags{{
		NodeID:   nodeID,
		Name:     nodes.OwnerTagName,
		Value:    nodes.EncodeOwner(userID),
		SignedAt: time.Now(),
		Signer:   satelliteID,
	}})
}

// memoryConsoleDB implements the part of console.DB the links use.
type memoryConsoleDB struct {
	console.DB

	users *memoryUsers
}

func (m *memoryConsoleDB) Users() console.Users { return m.users }

type memoryUsers struct {
	console.Users

	users map[uuid.UUID]*console.User
}

func (m *memoryUsers) Get(ctx context.Context, id uuid.UUID) (*console.User, error) {
	user, ok := m.users[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return user, nil
}

func (m *memoryUsers) add(email string) *console.User {
	user := &console.User{ID: testrand.UUID(), Email: email, Status: console.Active}
	m.users[user.ID] = user
	return user
}

// memoryChats implements Persistence in memory.
type memoryChats struct {
	mu    sync.Mutex
	chats map[uuid.UUID]int64
}

func newMemoryChats() *memoryChats {
	return &memoryChats{chats: map[uuid.UUID]int64{}}
}

func (m *memoryChats) GetChatID(ctx context.Context, user uuid.UUID) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.chats[user], nil
}

func (m *memoryChats) SaveChatID(ctx context.Context, user uuid.UUID, chatID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chats[user] = chatID
	return nil
}

func (m *memoryChats) DeleteChatID(ctx context.Context, user uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.chats, user)
	return nil
}

var satelliteID = testrand.NodeID()

// fakeBotAPI is a Telegram Bot API server, which records sent messages and
// hands out queued updates.
type fakeBotAPI struct {
	*httptest.Server

	mu       sync.Mutex
	sent     []sentMessage
	updates  []Update
	failSend bool
}

type sentMessage struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func newFakeBotAPI(t *testing.T, token string) *fakeBotAPI {
	f := &fakeBotAPI{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, ok := strings.CutPrefix(r.URL.Path, "/bot"+token+"/")
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"ok":false,"description":"Unauthorized"}`)
			return
		}

		f.mu.Lock()
		defer f.mu.Unlock()

		switch method {
		case "sendMessage":
			if f.failSend {
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, `{"ok":false,"description":"Forbidden: bot was blocked by the user"}`)
				return
			}
			var msg sentMessage
			_ = json.NewDecoder(r.Body).Decode(&msg)
			f.sent = append(f.sent, msg)
			_, _ = io.WriteString(w, `{"ok":true,"result":{}}`)
		case "getUpdates":
			var params struct {
				Offset int64 `json:"offset"`
			}
			_ = json.NewDecoder(r.Body).Decode(&params)
			var pending []Update
			for _, u := range f.updates {
				if u.UpdateID >= params.Offset {
					pending = append(pending, u)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": pending})
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"ok":false,"description":"Not Found"}`)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeBotAPI) messages() []sentMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMessage(nil), f.sent...)
}

type testEnv struct {
	overlay *memoryOverlay
	users   *memoryUsers
	api     *fakeBotAPI
	service *Service
}

func newTestEnv(t *testing.T) *testEnv {
	const token = "123:secret"
	env := &testEnv{
		overlay: newMemoryOverlay(),
		users:   &memoryUsers{users: map[uuid.UUID]*console.User{}},
		api:     newFakeBotAPI(t, token),
	}
	env.service = NewService(Config{
		BotToken:       token,
		BotName:        "storj_test_bot",
		APIURL:         env.api.URL,
		LinkExpiration: time.Hour,
		PollTimeout:    time.Second,
	}, NewLinks(env.overlay, &memoryConsoleDB{users: env.users}, newMemoryChats(), satelliteID))
	return env
}
