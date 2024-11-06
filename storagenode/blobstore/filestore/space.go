// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"storj.io/storj/storagenode/blobstore"
)

const infoMaxAge = time.Minute

type infoAge struct {
	info blobstore.DiskInfo
	age  time.Time
}

type CacheDirSpaceInfo struct {
	path string

	mu   sync.Mutex
	info atomic.Pointer[infoAge]
}

func NewCacheDirSpaceInfo(path string) *CacheDirSpaceInfo {
	return &CacheDirSpaceInfo{
		path: path,
	}
}

func (c *CacheDirSpaceInfo) AvailableSpace(ctx context.Context) (blobstore.DiskInfo, error) {
	if info := c.info.Load(); info != nil && time.Since(info.age) < infoMaxAge {
		return info.info, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if info := c.info.Load(); info != nil && time.Since(info.age) < infoMaxAge {
		return info.info, nil
	}

	path, err := filepath.Abs(c.path)
	if err != nil {
		return blobstore.DiskInfo{}, err
	}
	info, err := DiskInfoFromPath(path)
	if err != nil {
		return blobstore.DiskInfo{}, err
	}
	c.info.Store(&infoAge{info: info, age: time.Now()})
	return info, nil
}
