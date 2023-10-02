package common

import (
	"context"
	"storj.io/common/storj"
)

// Peer represents one of StorageNode or Satellite.
type Peer interface {
	Label() string

	ID() storj.NodeID
	Addr() string
	URL() string
	NodeURL() storj.NodeURL

	Run(context.Context) error
	Close() error
}
