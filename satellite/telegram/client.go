// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a minimal client of the Telegram Bot API, covering only the calls
// the satellite needs.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient creates a Bot API client. baseURL is usually https://api.telegram.org.
func NewClient(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: timeout},
	}
}

// Update is an incoming update, as returned by getUpdates.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// Message is a message sent to the bot.
type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

// Chat is the conversation a message belongs to.
type Chat struct {
	ID int64 `json:"id"`
}

// SendMessage sends a plain text message to a chat.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) (err error) {
	defer mon.Task()(&ctx)(&err)

	return c.call(ctx, "sendMessage", map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}, nil)
}

// GetUpdates long polls for updates with an ID of at least offset.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout time.Duration) (updates []Update, err error) {
	defer mon.Task()(&ctx)(&err)

	err = c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         int(timeout.Seconds()),
		"allowed_updates": []string{"message"},
	}, &updates)
	return updates, err
}

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return Error.Wrap(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+c.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return Error.New("%s: invalid request", method)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		// N.B. the request URL carries the bot token, and *url.Error prints the
		// URL. Keep only the cause so the token never ends up in the logs.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return Error.New("%s: %v", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var response struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 10<<20)).Decode(&response)
	if err != nil {
		return Error.New("%s: invalid response (HTTP %d): %v", method, resp.StatusCode, err)
	}
	if !response.OK {
		return Error.New("%s: %s (HTTP %d)", method, response.Description, resp.StatusCode)
	}

	if result != nil {
		if err := json.Unmarshal(response.Result, result); err != nil {
			return Error.New("%s: invalid result: %v", method, err)
		}
	}
	return nil
}
