// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

import (
	"context"

	"github.com/zeebo/errs"
)

// MultiNotifier implements Notifier by passing the events to all the notifiers
// registered in the []Notifier multibinding, for example both email and
// Telegram.
//
// It can be selected with --components=nodeevents.Notifier=nodeevents.MultiNotifier.
type MultiNotifier struct {
	notifiers []Notifier
}

var _ Notifier = (*MultiNotifier)(nil)

// NewMultiNotifier creates a notifier which notifies all of notifiers.
func NewMultiNotifier(notifiers []Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: notifiers}
}

// Notify implements Notifier. Every notifier is called, even if an earlier one
// failed, and the errors are combined.
//
// N.B. an error makes the chore retry the batch with all of the notifiers,
// including the ones which succeeded. Notifiers for which a duplicate is worse
// than a miss should handle their errors themselves.
func (m *MultiNotifier) Notify(ctx context.Context, satellite string, events []NodeEvent) (err error) {
	defer mon.Task()(&ctx)(&err)

	var group errs.Group
	for _, notifier := range m.notifiers {
		group.Add(notifier.Notify(ctx, satellite, events))
	}
	return group.Err()
}
