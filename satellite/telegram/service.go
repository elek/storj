// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package telegram notifies node operators about their nodes over Telegram.
//
// A console user connects a Telegram chat by opening a deep link to the bot,
// https://t.me/<bot>?start=<token>. The bot receives the token with the /start
// command and records the chat on the nodes the user has confirmed. From then
// on the node events of those nodes are sent to the chat as well.
package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"net/url"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

var (
	mon = monkit.Package()

	// Error is the error class for Telegram notifications.
	Error = errs.Class("telegram")
)

// Config configures the Telegram bot.
type Config struct {
	BotToken       string        `help:"token of the Telegram bot used to notify node operators; Telegram notifications are disabled when empty" default:""`
	BotName        string        `help:"username of the Telegram bot, without the leading @" default:""`
	APIURL         string        `help:"base URL of the Telegram Bot API" default:"https://api.telegram.org"`
	LinkExpiration time.Duration `help:"how long a link connecting a console account to a Telegram chat is valid" default:"1h"`
	PollTimeout    time.Duration `help:"how long the bot waits for new messages in a single request" default:"50s"`
}

// Service is the Telegram bot as seen by the rest of the satellite: it sends
// messages, and creates and checks the links that connect a chat to a user.
type Service struct {
	config Config
	client *Client
	links  *Links
	nowFn  func() time.Time
}

// NewService creates the Telegram service.
func NewService(config Config, links *Links) *Service {
	return &Service{
		config: config,
		// the HTTP timeout has to outlast a long poll
		client: NewClient(config.APIURL, config.BotToken, config.PollTimeout+30*time.Second),
		links:  links,
		nowFn:  time.Now,
	}
}

// Enabled returns whether a bot is configured.
func (s *Service) Enabled() bool {
	return s.config.BotToken != "" && s.config.BotName != ""
}

// BotName returns the username of the bot.
func (s *Service) BotName() string {
	return s.config.BotName
}

// Client returns the Bot API client.
func (s *Service) Client() *Client {
	return s.client
}

// Links returns the store of chat links.
func (s *Service) Links() *Links {
	return s.links
}

// LinkURL returns the deep link which connects the chat it is opened in to the user.
func (s *Service) LinkURL(userID uuid.UUID) string {
	return "https://t.me/" + url.PathEscape(s.config.BotName) + "?start=" + s.linkToken(userID, s.nowFn().Add(s.config.LinkExpiration))
}

// linkToken encodes the user and the expiration, authenticated with a key
// derived from the bot token. Telegram passes the start parameter back as is;
// it may only contain [A-Za-z0-9_-] and be at most 64 characters, which the
// 54 characters of raw URL base64 of 40 bytes satisfy.
func (s *Service) linkToken(userID uuid.UUID, expires time.Time) string {
	payload := binary.BigEndian.AppendUint64(userID.Bytes(), uint64(expires.Unix()))
	return base64.RawURLEncoding.EncodeToString(append(payload, s.mac(payload)...))
}

// VerifyLinkToken returns the user a link token was created for.
func (s *Service) VerifyLinkToken(token string) (uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != len(uuid.UUID{})+8+macSize {
		return uuid.UUID{}, Error.New("invalid link")
	}
	payload, mac := raw[:len(raw)-macSize], raw[len(raw)-macSize:]
	if !hmac.Equal(mac, s.mac(payload)) {
		return uuid.UUID{}, Error.New("invalid link")
	}
	expires := time.Unix(int64(binary.BigEndian.Uint64(payload[len(uuid.UUID{}):])), 0)
	if s.nowFn().After(expires) {
		return uuid.UUID{}, Error.New("expired link")
	}
	userID, err := uuid.FromBytes(payload[:len(uuid.UUID{})])
	return userID, Error.Wrap(err)
}

const macSize = 16

func (s *Service) mac(payload []byte) []byte {
	key := sha256.Sum256([]byte("storj satellite telegram link\x00" + s.config.BotToken))
	h := hmac.New(sha256.New, key[:])
	_, _ = h.Write(payload)
	return h.Sum(nil)[:macSize]
}
