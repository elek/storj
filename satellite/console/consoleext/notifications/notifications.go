// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package notifications lets the logged in console user choose where the
// notifications about their confirmed nodes are sent, in addition to email.
package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleext"
	"storj.io/storj/satellite/telegram"
)

var (
	mon = monkit.Package()

	// Error is the error class for the console notifications extension.
	Error = errs.Class("console notifications")
)

// Settings is the response of the notifications endpoint.
type Settings struct {
	Telegram TelegramSettings `json:"telegram"`
}

// TelegramSettings describes the Telegram chat of the user.
type TelegramSettings struct {
	// Enabled is false when the satellite has no Telegram bot configured.
	Enabled bool `json:"enabled"`
	// BotName is the username of the bot, if Enabled.
	BotName string `json:"botName"`
	// Connected is true when the user has connected a chat.
	Connected bool `json:"connected"`
}

// testInterval is how often a user may send a test message. Telegram rate
// limits per bot, so a user looping the test would delay everybody's
// notifications.
const testInterval = 30 * time.Second

// Extension serves the notification settings of the logged in user.
type Extension struct {
	log      *zap.Logger
	telegram *telegram.Service
	nowFn    func() time.Time

	mu       sync.Mutex
	lastTest map[uuid.UUID]time.Time
}

// New creates the notifications console extension.
func New(log *zap.Logger, telegram *telegram.Service) *Extension {
	return &Extension{
		log:      log,
		telegram: telegram,
		nowFn:    time.Now,
		lastTest: map[uuid.UUID]time.Time{},
	}
}

// Name implements consoleext.Extension.
func (e *Extension) Name() string { return "notifications" }

// Register implements consoleext.Extension.
func (e *Extension) Register(router *mux.Router, deps consoleext.Deps) {
	notificationsRouter := router.PathPrefix("/api/v0/notifications").Subrouter()
	notificationsRouter.Use(deps.WithAuth)
	notificationsRouter.Handle("", http.HandlerFunc(e.GetSettings)).Methods(http.MethodGet, http.MethodOptions)
	notificationsRouter.Handle("/telegram/link", http.HandlerFunc(e.CreateTelegramLink)).Methods(http.MethodPost, http.MethodOptions)
	notificationsRouter.Handle("/telegram/test", http.HandlerFunc(e.TestTelegram)).Methods(http.MethodPost, http.MethodOptions)
	notificationsRouter.Handle("/telegram", http.HandlerFunc(e.DisconnectTelegram)).Methods(http.MethodDelete, http.MethodOptions)
}

// GetSettings returns the notification settings of the user.
func (e *Extension) GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	user, status, err := activeUser(ctx)
	if err != nil {
		e.serveJSONError(ctx, w, status, err)
		return
	}

	settings := Settings{Telegram: TelegramSettings{Enabled: e.telegram.Enabled()}}
	if settings.Telegram.Enabled {
		settings.Telegram.BotName = e.telegram.BotName()
		_, settings.Telegram.Connected, err = e.telegram.Links().ChatOf(ctx, user.ID)
		if err != nil {
			e.serveJSONError(ctx, w, http.StatusInternalServerError, err)
			return
		}
	}

	e.serveJSON(ctx, w, settings)
}

// CreateTelegramLink returns a deep link to the bot, which connects the chat it
// is opened in to the user.
func (e *Extension) CreateTelegramLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	user, status, err := activeUser(ctx)
	if err != nil {
		e.serveJSONError(ctx, w, status, err)
		return
	}
	if !e.telegram.Enabled() {
		e.serveJSONError(ctx, w, http.StatusNotFound, Error.New("telegram notifications are not available"))
		return
	}

	e.serveJSON(ctx, w, struct {
		URL string `json:"url"`
	}{URL: e.telegram.LinkURL(user.ID)})
}

// TestTelegram sends a test message to the chat of the user.
func (e *Extension) TestTelegram(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	user, status, err := activeUser(ctx)
	if err != nil {
		e.serveJSONError(ctx, w, status, err)
		return
	}
	if !e.telegram.Enabled() {
		e.serveJSONError(ctx, w, http.StatusNotFound, Error.New("telegram notifications are not available"))
		return
	}

	chatID, connected, err := e.telegram.Links().ChatOf(ctx, user.ID)
	if err != nil {
		e.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}
	if !connected {
		e.serveJSONError(ctx, w, http.StatusConflict, Error.New("no telegram chat is connected"))
		return
	}
	if !e.allowTest(user.ID) {
		e.serveJSONError(ctx, w, http.StatusTooManyRequests, Error.New("please wait before sending another test message"))
		return
	}

	err = e.telegram.Client().SendMessage(ctx, chatID, "This is a test notification. Notifications about your confirmed storage nodes will arrive in this chat.")
	if err != nil {
		e.log.Warn("failed to send telegram test message", zap.Error(err))
		e.serveJSONError(ctx, w, http.StatusBadGateway, Error.New("failed to send the message to telegram"))
		return
	}

	e.serveJSON(ctx, w, struct {
		Sent bool `json:"sent"`
	}{Sent: true})
}

// DisconnectTelegram disconnects the chat of the user.
func (e *Extension) DisconnectTelegram(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	user, status, err := activeUser(ctx)
	if err != nil {
		e.serveJSONError(ctx, w, status, err)
		return
	}

	err = e.telegram.Links().Unlink(ctx, user.ID)
	if err != nil {
		e.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	e.serveJSON(ctx, w, struct {
		Disconnected bool `json:"disconnected"`
	}{Disconnected: true})
}

// allowTest reports whether the user may send a test message now, and if so,
// records it.
func (e *Extension) allowTest(userID uuid.UUID) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.nowFn()
	for id, last := range e.lastTest {
		if now.Sub(last) >= testInterval {
			delete(e.lastTest, id)
		}
	}
	if _, ok := e.lastTest[userID]; ok {
		return false
	}
	e.lastTest[userID] = now
	return true
}

// activeUser returns the logged in user along with the HTTP status to report if
// they may not manage notifications. Nodes are only resolved for users who
// verified their email address, see consoleext/nodes.
func activeUser(ctx context.Context) (*console.User, int, error) {
	user, err := console.GetUser(ctx)
	if err != nil {
		return nil, http.StatusUnauthorized, err
	}
	if user.Status != console.Active {
		return nil, http.StatusForbidden, Error.New("email address is not verified")
	}
	return user, http.StatusOK, nil
}

func (e *Extension) serveJSON(ctx context.Context, w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		e.log.Error("failed to write json response", zap.Error(Error.Wrap(err)))
	}
}

func (e *Extension) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, e.log, w, status, err)
}
