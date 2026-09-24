// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package telegram

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/nodeevents"
)

func TestLinkToken(t *testing.T) {
	env := newTestEnv(t)
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	env.service.nowFn = func() time.Time { return now }

	userID := testrand.UUID()
	link := env.service.LinkURL(userID)

	token, ok := strings.CutPrefix(link, "https://t.me/storj_test_bot?start=")
	require.True(t, ok, link)
	require.LessOrEqual(t, len(token), 64, "telegram accepts at most 64 characters")
	require.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9_-]+$`), token, "telegram accepts only these characters")

	got, err := env.service.VerifyLinkToken(token)
	require.NoError(t, err)
	require.Equal(t, userID, got)

	t.Run("tampered", func(t *testing.T) {
		tampered := []byte(token)
		if tampered[3] == 'A' {
			tampered[3] = 'B'
		} else {
			tampered[3] = 'A'
		}
		_, err := env.service.VerifyLinkToken(string(tampered))
		require.Error(t, err)
	})

	t.Run("garbage", func(t *testing.T) {
		for _, garbage := range []string{"", "x", "!!!", token[:len(token)-1]} {
			_, err := env.service.VerifyLinkToken(garbage)
			require.Error(t, err, garbage)
		}
	})

	t.Run("other bot", func(t *testing.T) {
		other := NewService(Config{BotToken: "456:other", BotName: "x", LinkExpiration: time.Hour}, nil)
		other.nowFn = env.service.nowFn
		_, err := other.VerifyLinkToken(token)
		require.Error(t, err)
	})

	t.Run("expired", func(t *testing.T) {
		env.service.nowFn = func() time.Time { return now.Add(time.Hour + time.Second) }
		defer func() { env.service.nowFn = func() time.Time { return now } }()
		_, err := env.service.VerifyLinkToken(token)
		require.Error(t, err)
	})
}

func TestLinks(t *testing.T) {
	ctx := testcontext.New(t)
	env := newTestEnv(t)
	links := env.service.Links()

	user := env.users.add("operator@storj.test")
	owned1 := env.overlay.addNode("Operator@Storj.Test")
	owned2 := env.overlay.addNode("operator@storj.test")
	unconfirmed := env.overlay.addNode("operator@storj.test")
	env.overlay.confirm(owned1, user.ID)
	env.overlay.confirm(owned2, user.ID)

	_, ok, err := links.ChatOf(ctx, user.ID)
	require.NoError(t, err)
	require.False(t, ok)

	count, err := links.Link(ctx, user.ID, 1001)
	require.NoError(t, err)
	require.Equal(t, 2, count, "only the confirmed nodes are counted")

	tags, err := env.overlay.GetNodeTags(ctx, owned1)
	require.NoError(t, err)
	require.Len(t, tags, 1, "the link is not stored in node tags")
	tags, err = env.overlay.GetNodeTags(ctx, unconfirmed)
	require.NoError(t, err)
	require.Empty(t, tags)

	chatID, ok, err := links.ChatOf(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.EqualValues(t, 1001, chatID)

	// connecting another chat replaces the first one
	_, err = links.Link(ctx, user.ID, -2002)
	require.NoError(t, err)
	chatID, ok, err = links.ChatOf(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.EqualValues(t, -2002, chatID)

	require.NoError(t, links.Unlink(ctx, user.ID))
	_, ok, err = links.ChatOf(ctx, user.ID)
	require.NoError(t, err)
	require.False(t, ok)

	t.Run("no confirmed nodes", func(t *testing.T) {
		other := env.users.add("other@storj.test")
		env.overlay.addNode("other@storj.test")
		_, err := links.Link(ctx, other.ID, 3003)
		require.ErrorIs(t, err, ErrNoOwnedNodes)
	})

	t.Run("links of a previous owner are ignored", func(t *testing.T) {
		// both accounts registered with the same address; the second one took
		// over the node after the first one connected a chat.
		previous := env.users.add("shared@storj.test")
		current := env.users.add("SHARED@storj.test")
		node := env.overlay.addNode("shared@storj.test")
		env.overlay.confirm(node, previous.ID)
		_, err := links.Link(ctx, previous.ID, 4004)
		require.NoError(t, err)

		env.overlay.confirm(node, current.ID)
		owner, err := links.OwnerOf(ctx, node)
		require.NoError(t, err)
		require.Equal(t, current.ID, owner)

		_, ok, err := links.ChatOf(ctx, current.ID)
		require.NoError(t, err)
		require.False(t, ok, "the chat of the previous owner must not be used for the new one")
	})
}

func TestNotifier(t *testing.T) {
	ctx := testcontext.New(t)
	env := newTestEnv(t)
	notifier := NewNotifier(zaptest.NewLogger(t), env.service)

	user := env.users.add("operator@storj.test")
	linked := env.overlay.addNode("operator@storj.test")
	env.overlay.confirm(linked, user.ID)
	_, err := env.service.Links().Link(ctx, user.ID, 1001)
	require.NoError(t, err)

	// confirmed after the chat was connected: still notified
	later := env.overlay.addNode("operator@storj.test")
	env.overlay.confirm(later, user.ID)

	// same email, never confirmed: not notified
	unconfirmed := env.overlay.addNode("operator@storj.test")

	// confirmed by somebody without a chat
	lonely := env.users.add("lonely@storj.test")
	unlinked := env.overlay.addNode("lonely@storj.test")
	env.overlay.confirm(unlinked, lonely.ID)

	events := func(event nodeevents.Type, ids ...storj.NodeID) (result []nodeevents.NodeEvent) {
		for _, id := range ids {
			result = append(result, nodeevents.NodeEvent{ID: testrand.UUID(), Email: "operator@storj.test", NodeID: id, Event: event})
		}
		return result
	}

	err = notifier.Notify(ctx, "Test Satellite", events(nodeevents.Offline, linked, later, linked, unconfirmed, unlinked))
	require.NoError(t, err)

	sent := env.api.messages()
	require.Len(t, sent, 1)
	require.EqualValues(t, 1001, sent[0].ChatID)
	require.Equal(t, "Satellite Test Satellite:\n\nNode "+linked.String()+" is offline.\nNode "+later.String()+" is offline.", sent[0].Text)

	t.Run("send failures do not fail the batch", func(t *testing.T) {
		env.api.mu.Lock()
		env.api.failSend = true
		env.api.mu.Unlock()
		defer func() {
			env.api.mu.Lock()
			env.api.failSend = false
			env.api.mu.Unlock()
		}()

		require.NoError(t, notifier.Notify(ctx, "Test Satellite", events(nodeevents.Online, linked)))
	})

	t.Run("disabled", func(t *testing.T) {
		disabled := NewNotifier(zaptest.NewLogger(t), NewService(Config{}, nil))
		require.NoError(t, disabled.Notify(ctx, "Test Satellite", events(nodeevents.Online, linked)))
	})
}

func TestMessages(t *testing.T) {
	require.Empty(t, messages("sat", nil))

	line := strings.Repeat("x", 1000)
	var lines []string
	for range 9 {
		lines = append(lines, line)
	}

	result := messages("sat", lines)
	require.Len(t, result, 3)
	var total int
	for _, msg := range result {
		require.LessOrEqual(t, len(msg), maxMessageLength)
		require.True(t, strings.HasPrefix(msg, "Satellite sat:\n"))
		total += strings.Count(msg, line)
	}
	require.Equal(t, len(lines), total, "no line is lost")
}

func TestBot(t *testing.T) {
	ctx := testcontext.New(t)
	env := newTestEnv(t)
	bot := NewBot(zaptest.NewLogger(t), env.service)

	user := env.users.add("operator@storj.test")
	node := env.overlay.addNode("operator@storj.test")
	env.overlay.confirm(node, user.ID)

	token := strings.TrimPrefix(env.service.LinkURL(user.ID), "https://t.me/storj_test_bot?start=")

	env.api.mu.Lock()
	env.api.updates = []Update{
		{UpdateID: 10, Message: &Message{MessageID: 1, Chat: Chat{ID: 1001}, Text: "/start"}},
		{UpdateID: 11, Message: &Message{MessageID: 2, Chat: Chat{ID: 1001}, Text: "/start invalid"}},
		{UpdateID: 12, Message: &Message{MessageID: 3, Chat: Chat{ID: 1001}, Text: "/start " + token}},
		{UpdateID: 13, Message: &Message{MessageID: 4, Chat: Chat{ID: 1001}, Text: "hello"}},
	}
	env.api.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- bot.Run(runCtx) }()

	require.Eventually(t, func() bool { return len(env.api.messages()) >= 4 }, 10*time.Second, 10*time.Millisecond)
	cancel()
	require.NoError(t, <-done)

	sent := env.api.messages()
	require.Len(t, sent, 4, "every update is answered once")
	require.Contains(t, sent[0].Text, "Notifications page")
	require.Contains(t, sent[1].Text, "invalid or expired")
	require.Contains(t, sent[2].Text, "Done!")
	require.Contains(t, sent[3].Text, "Notifications page")

	chatID, ok, err := env.service.Links().ChatOf(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.EqualValues(t, 1001, chatID)

	t.Run("disabled", func(t *testing.T) {
		disabled := NewBot(zaptest.NewLogger(t), NewService(Config{}, nil))
		require.NoError(t, disabled.Run(ctx))
	})
}

func TestClientHidesToken(t *testing.T) {
	ctx := testcontext.New(t)

	// nothing listens on this port; the connection error must not carry the URL.
	client := NewClient("http://127.0.0.1:1", "123:very-secret", time.Second)
	err := client.SendMessage(ctx, 1, "hi")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "very-secret")
}
