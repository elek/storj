// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeinvites

// NodeOperatorInviteEmail is a mailservice template inviting a storage node operator
// to create a console account.
type NodeOperatorInviteEmail struct {
	SignUpLink string
	NodeCount  int
}

// Template returns email template name.
func (*NodeOperatorInviteEmail) Template() string { return "NodeOperatorInvite" }

// Subject gets email subject.
func (*NodeOperatorInviteEmail) Subject() string { return "Store your own data on your nodes" }
