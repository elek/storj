// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package emails embeds the satellite email templates, making them available
// to the satellite binary without shipping the source tree.
package emails

import (
	"embed"
)

// Templates contains the HTML and plain text email templates of this directory.
//
//go:embed *.html *.txt
var Templates embed.FS
