// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"

	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
)

type TrustedSatelliteSource interface {
	GetSatellites(ctx context.Context) (satellites []storj.NodeID)
	GetNodeURL(ctx context.Context, id storj.NodeID) (_ storj.NodeURL, err error)
	VerifySatelliteID(ctx context.Context, id storj.NodeID) error
	GetSignee(ctx context.Context, id pb.NodeID) (signing.Signee, error)
}
