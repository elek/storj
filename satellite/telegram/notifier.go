// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package telegram

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/nodeevents"
)

// maxMessageLength stays below the 4096 characters Telegram accepts in one message.
const maxMessageLength = 4000

// Notifier implements nodeevents.Notifier. It sends the events of confirmed
// nodes to the Telegram chat their owner connected.
type Notifier struct {
	log     *zap.Logger
	service *Service
}

var _ nodeevents.Notifier = (*Notifier)(nil)

// NewNotifier creates the Telegram notifier.
func NewNotifier(log *zap.Logger, service *Service) *Notifier {
	return &Notifier{log: log, service: service}
}

// Notify implements nodeevents.Notifier.
//
// Sending is best-effort: a chat that cannot be reached is logged, not
// reported, since the chore would retry the whole batch, including the chats
// (and, with MultiNotifier, the emails) which were already notified. Database
// errors are reported, as nothing has been sent by the time they can happen.
func (n *Notifier) Notify(ctx context.Context, satellite string, events []nodeevents.NodeEvent) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !n.service.Enabled() || len(events) == 0 {
		return nil
	}

	chats := map[uuid.UUID]int64{}
	lines := map[int64][]string{}
	var order []int64
	seen := map[storj.NodeID]struct{}{}
	for _, event := range events {
		if _, ok := seen[event.NodeID]; ok {
			continue
		}
		seen[event.NodeID] = struct{}{}

		owner, err := n.service.Links().OwnerOf(ctx, event.NodeID)
		if err != nil {
			return err
		}
		if owner.IsZero() {
			continue
		}

		// chat ID 0 is never a valid Telegram chat, it marks owners without one.
		chatID, ok := chats[owner]
		if !ok {
			chatID, _, err = n.service.Links().ChatOf(ctx, owner)
			if err != nil {
				return err
			}
			chats[owner] = chatID
		}
		if chatID == 0 {
			continue
		}

		if _, ok := lines[chatID]; !ok {
			order = append(order, chatID)
		}
		lines[chatID] = append(lines[chatID], fmt.Sprintf("Node %s %s.", event.NodeID, describe(event.Event)))
	}

	for _, chatID := range order {
		for _, message := range messages(satellite, lines[chatID]) {
			if err := n.service.Client().SendMessage(ctx, chatID, message); err != nil {
				n.log.Warn("failed to send telegram notification", zap.Int64("chat", chatID), zap.Error(err))
				break
			}
		}
	}
	return nil
}

// messages joins the lines under a header, splitting them into as many
// messages as needed to fit the Telegram limit.
func messages(satellite string, lines []string) []string {
	header := "Satellite " + satellite + ":\n"

	var result []string
	var current strings.Builder
	for _, line := range lines {
		if current.Len() > 0 && current.Len()+len(line)+1 > maxMessageLength {
			result = append(result, current.String())
			current.Reset()
		}
		if current.Len() == 0 {
			current.WriteString(header)
		}
		current.WriteString("\n")
		current.WriteString(line)
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

func describe(event nodeevents.Type) string {
	switch event {
	case nodeevents.Online:
		return "is back online"
	case nodeevents.Offline:
		return "is offline"
	case nodeevents.Disqualified:
		return "has been disqualified"
	case nodeevents.UnknownAuditSuspended:
		return "is suspended for unknown audit errors"
	case nodeevents.UnknownAuditUnsuspended:
		return "is no longer suspended for unknown audit errors"
	case nodeevents.OfflineSuspended:
		return "is suspended for being offline"
	case nodeevents.OfflineUnsuspended:
		return "is no longer suspended for being offline"
	case nodeevents.BelowMinVersion:
		return "runs a version below the minimum allowed"
	default:
		name, err := event.Name()
		if err != nil {
			return "has a new event"
		}
		return "has a new event: " + name
	}
}
