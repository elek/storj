// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/nodeevents"
)

func TestMultiNotifier(t *testing.T) {
	ctx := testcontext.New(t)

	events := []nodeevents.NodeEvent{{ID: testrand.UUID(), Email: "operator@storj.test", NodeID: testrand.NodeID(), Event: nodeevents.Offline}}

	first := &TestNotifier{notifications: map[string][]nodeevents.NodeEvent{}}
	failing := &ErrorNotifier{}
	last := &TestNotifier{notifications: map[string][]nodeevents.NodeEvent{}}

	err := nodeevents.NewMultiNotifier([]nodeevents.Notifier{first, failing, last}).Notify(ctx, "sat", events)
	require.Error(t, err)

	require.Equal(t, events, first.notifications["operator@storj.test"])
	require.Equal(t, 1, failing.errCount)
	require.Equal(t, events, last.notifications["operator@storj.test"], "a failing notifier does not stop the others")

	require.NoError(t, nodeevents.NewMultiNotifier(nil).Notify(ctx, "sat", events))
}
