// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// Bot answers the messages sent to the Telegram bot. Its only job is to
// connect chats to console users, through the /start command of a deep link.
//
// Telegram allows only one long poll per bot token, so exactly one satellite
// process should run the bot.
type Bot struct {
	log     *zap.Logger
	service *Service
}

// NewBot creates the Telegram bot.
func NewBot(log *zap.Logger, service *Service) *Bot {
	return &Bot{log: log, service: service}
}

// Run polls for messages until ctx is canceled.
func (b *Bot) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !b.service.Enabled() {
		b.log.Info("telegram bot is not configured; not polling for messages")
		return nil
	}

	var offset int64
	for {
		updates, err := b.service.Client().GetUpdates(ctx, offset, b.service.config.PollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			b.log.Warn("failed to poll telegram updates", zap.Error(err))
			if !sync2.Sleep(ctx, 10*time.Second) {
				return nil
			}
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			if update.Message == nil {
				continue
			}
			b.handle(ctx, update.Message)
		}
	}
}

func (b *Bot) handle(ctx context.Context, msg *Message) {
	answer := b.answer(ctx, msg)
	if err := b.service.Client().SendMessage(ctx, msg.Chat.ID, answer); err != nil {
		b.log.Warn("failed to answer telegram message", zap.Error(err))
	}
}

func (b *Bot) answer(ctx context.Context, msg *Message) string {
	command, argument, _ := strings.Cut(strings.TrimSpace(msg.Text), " ")
	// in groups commands can be addressed as /start@botname
	command, _, _ = strings.Cut(command, "@")

	switch command {
	case "/start":
		argument = strings.TrimSpace(argument)
		if argument == "" {
			return "Hi! I send notifications about your storage nodes. Open the Notifications page of the satellite console to connect this chat to your account."
		}
		return b.link(ctx, msg.Chat.ID, argument)
	default:
		return "To connect this chat, open the Notifications page of the satellite console. You can disconnect it on the same page."
	}
}

func (b *Bot) link(ctx context.Context, chatID int64, token string) string {
	userID, err := b.service.VerifyLinkToken(token)
	if err != nil {
		return "This link is invalid or expired. Please request a new one on the Notifications page of the satellite console."
	}

	count, err := b.service.Links().Link(ctx, userID, chatID)
	if err != nil {
		if errors.Is(err, ErrNoOwnedNodes) {
			return "You don't have any confirmed nodes yet. Confirm your nodes on the Nodes page of the satellite console, then connect this chat again."
		}
		b.log.Error("failed to link telegram chat", zap.Stringer("user", userID), zap.Error(err))
		return "Sorry, something went wrong. Please try again later."
	}

	if count == 1 {
		return "Done! You will get notifications about your node here."
	}
	return fmt.Sprintf("Done! You will get notifications about your %d nodes here.", count)
}
