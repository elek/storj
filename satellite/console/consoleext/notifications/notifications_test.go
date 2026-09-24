// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleext"
	"storj.io/storj/satellite/console/consoleext/nodes"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/telegram"
)

var satelliteID = testrand.NodeID()

// stubOverlayDB holds a single node of the user.
type stubOverlayDB struct {
	overlay.DB

	mu   sync.Mutex
	node *overlay.NodeDossier
}

func (s *stubOverlayDB) GetNodesByEmailInsensitive(ctx context.Context, email string, limit int) ([]*overlay.NodeDossier, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !strings.EqualFold(email, s.node.Operator.Email) {
		return nil, nil
	}
	d := *s.node
	d.Tags = append(nodeselection.NodeTags(nil), s.node.Tags...)
	return []*overlay.NodeDossier{&d}, nil
}

// stubChats implements telegram.Persistence in memory.
type stubChats struct {
	mu    sync.Mutex
	chats map[uuid.UUID]int64
}

func (s *stubChats) GetChatID(ctx context.Context, user uuid.UUID) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.chats[user], nil
}

func (s *stubChats) SaveChatID(ctx context.Context, user uuid.UUID, chatID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chats[user] = chatID
	return nil
}

func (s *stubChats) DeleteChatID(ctx context.Context, user uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.chats, user)
	return nil
}

type stubConsoleDB struct {
	console.DB

	user *console.User
}

func (s *stubConsoleDB) Users() console.Users { return stubUsers{user: s.user} }

type stubUsers struct {
	console.Users
	user *console.User
}

func (s stubUsers) Get(ctx context.Context, id uuid.UUID) (*console.User, error) {
	return s.user, nil
}

type env struct {
	ext     *Extension
	user    *console.User
	overlay *stubOverlayDB
	links   *telegram.Links
	sent    chan string
}

func newEnv(t *testing.T, cfg telegram.Config) *env {
	user := &console.User{ID: testrand.UUID(), Email: "operator@storj.test", Status: console.Active}
	nodeID := testrand.NodeID()
	overlayDB := &stubOverlayDB{node: &overlay.NodeDossier{
		Node:     pb.Node{Id: nodeID},
		Operator: pb.NodeOperator{Email: user.Email},
		Tags: nodeselection.NodeTags{{
			NodeID: nodeID, Name: nodes.OwnerTagName, Value: nodes.EncodeOwner(user.ID), SignedAt: time.Now(), Signer: satelliteID,
		}},
	}}

	sent := make(chan string, 10)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&msg)
		sent <- msg.Text
		_, _ = io.WriteString(w, `{"ok":true,"result":{}}`)
	}))
	t.Cleanup(api.Close)
	cfg.APIURL = api.URL

	links := telegram.NewLinks(overlayDB, &stubConsoleDB{user: user}, &stubChats{chats: map[uuid.UUID]int64{}}, satelliteID)
	return &env{
		ext:     New(zaptest.NewLogger(t), telegram.NewService(cfg, links)),
		user:    user,
		overlay: overlayDB,
		links:   links,
		sent:    sent,
	}
}

func (e *env) serve(t *testing.T, method, path string, user *console.User) (int, map[string]any) {
	t.Helper()

	router := mux.NewRouter()
	e.ext.Register(router, consoleext.Deps{WithAuth: func(next http.Handler) http.Handler { return next }})

	req := httptest.NewRequestWithContext(context.Background(), method, path, nil)
	if user != nil {
		req = req.WithContext(console.WithUser(req.Context(), user))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return rec.Code, body
}

var enabled = telegram.Config{BotToken: "123:secret", BotName: "storj_test_bot", LinkExpiration: time.Hour}

func TestSettings(t *testing.T) {
	t.Run("telegram disabled", func(t *testing.T) {
		e := newEnv(t, telegram.Config{})
		status, body := e.serve(t, http.MethodGet, "/api/v0/notifications", e.user)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, map[string]any{"enabled": false, "botName": "", "connected": false}, body["telegram"])

		status, _ = e.serve(t, http.MethodPost, "/api/v0/notifications/telegram/link", e.user)
		require.Equal(t, http.StatusNotFound, status)
	})

	t.Run("connect, test, disconnect", func(t *testing.T) {
		e := newEnv(t, enabled)

		status, body := e.serve(t, http.MethodGet, "/api/v0/notifications", e.user)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, map[string]any{"enabled": true, "botName": "storj_test_bot", "connected": false}, body["telegram"])

		status, body = e.serve(t, http.MethodPost, "/api/v0/notifications/telegram/link", e.user)
		require.Equal(t, http.StatusOK, status)
		require.True(t, strings.HasPrefix(body["url"].(string), "https://t.me/storj_test_bot?start="), body["url"])

		status, _ = e.serve(t, http.MethodPost, "/api/v0/notifications/telegram/test", e.user)
		require.Equal(t, http.StatusConflict, status, "nothing to test without a chat")

		// what the bot does when the link is opened
		_, err := e.links.Link(context.Background(), e.user.ID, 1001)
		require.NoError(t, err)

		_, body = e.serve(t, http.MethodGet, "/api/v0/notifications", e.user)
		require.Equal(t, true, body["telegram"].(map[string]any)["connected"])

		status, body = e.serve(t, http.MethodPost, "/api/v0/notifications/telegram/test", e.user)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, true, body["sent"])
		require.Contains(t, <-e.sent, "test notification")

		status, _ = e.serve(t, http.MethodPost, "/api/v0/notifications/telegram/test", e.user)
		require.Equal(t, http.StatusTooManyRequests, status, "test messages are rate limited")

		later := time.Now().Add(testInterval)
		e.ext.nowFn = func() time.Time { return later }
		status, _ = e.serve(t, http.MethodPost, "/api/v0/notifications/telegram/test", e.user)
		require.Equal(t, http.StatusOK, status)
		<-e.sent

		status, _ = e.serve(t, http.MethodDelete, "/api/v0/notifications/telegram", e.user)
		require.Equal(t, http.StatusOK, status)

		_, body = e.serve(t, http.MethodGet, "/api/v0/notifications", e.user)
		require.Equal(t, false, body["telegram"].(map[string]any)["connected"])
	})

	t.Run("unverified user", func(t *testing.T) {
		e := newEnv(t, enabled)
		inactive := *e.user
		inactive.Status = console.Inactive
		for _, req := range []struct{ method, path string }{
			{http.MethodGet, "/api/v0/notifications"},
			{http.MethodPost, "/api/v0/notifications/telegram/link"},
			{http.MethodPost, "/api/v0/notifications/telegram/test"},
			{http.MethodDelete, "/api/v0/notifications/telegram"},
		} {
			status, _ := e.serve(t, req.method, req.path, &inactive)
			require.Equal(t, http.StatusForbidden, status, req.path)
		}
	})

	t.Run("anonymous", func(t *testing.T) {
		e := newEnv(t, enabled)
		status, _ := e.serve(t, http.MethodGet, "/api/v0/notifications", nil)
		require.Equal(t, http.StatusUnauthorized, status)
	})
}
