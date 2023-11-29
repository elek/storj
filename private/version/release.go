// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/storj/shared/version.buildTimestamp
var buildTimestamp string

//go:linkname buildCommitHash storj.io/storj/shared/version.buildCommitHash
var buildCommitHash string

//go:linkname buildVersion storj.io/storj/shared/version.buildVersion
var buildVersion string

//go:linkname buildRelease storj.io/storj/shared/version.buildRelease
var buildRelease string

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
