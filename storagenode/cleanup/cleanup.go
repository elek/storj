// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cleanup

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/pieces"
)

// Cleanup is a service, which combines retain,TTL,trash, and runs only once (if load is not too high)
type Cleanup struct {
	log   *zap.Logger
	blobs blobstore.Blobs
	loop  *SafeLoop

	trashExpiryInterval time.Duration
}

// NewCleanup creates a new Cleanup.
func NewCleanup(log *zap.Logger, loop *SafeLoop, blobs blobstore.Blobs) *Cleanup {
	return &Cleanup{
		loop: loop,
		// TODO: change if it's configurable. Right now it's hardcoded in storagenode/peer.go, what we wouldn't like to depends on.
		trashExpiryInterval: 7 * 24 * time.Hour,
		blobs:               blobs,
		log:                 log,
	}
}

// Run starts running RunOnce in the safe loop.
func (c *Cleanup) Run(ctx context.Context) error {
	return c.loop.RunSafe(ctx, c.RunOnce)
}

// RunOnce executes all (TODO) chores, one by one. Can be stopped with cancelling context.
func (c *Cleanup) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = c.EmptyTrash(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// TODO: run all the other chores as well (retain, expire)
	return nil

}

func (c *Cleanup) EmptyTrash(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	namespaces, err := c.blobs.ListNamespaces(ctx)
	if err != nil {
		c.log.Error("couldn't get list of namespaces", zap.Error(err))
		return nil
	}
	for _, namespace := range namespaces {
		var satellite storj.NodeID
		copy(satellite[:], namespace)
		timeStart := time.Now()
		c.log.Info("emptying trash started", zap.Stringer("Satellite ID", satellite))
		trashedBefore := time.Now().Add(-c.trashExpiryInterval)
		if ws, ok := c.blobs.(pieces.SupportEmptyTrashWithoutStat); ok {
			err = ws.EmptyTrashWithoutStat(ctx, namespace, trashedBefore)
		} else {
			_, _, err = c.blobs.EmptyTrash(ctx, namespace, trashedBefore)
		}
		if err != nil {
			c.log.Error("emptying trash failed", zap.Error(err))
		} else {
			c.log.Debug("emptying trash finished", zap.Stringer("Satellite ID", satellite), zap.Duration("elapsed", time.Since(timeStart)))
		}

	}
	return nil
}
