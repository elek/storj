// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"

	"storj.io/common/storj"
	"storj.io/storj/private/api"
)

// PlacementInfo contains the ID and location of a placement rule.
type PlacementInfo struct {
	ID       storj.PlacementConstraint `json:"id"`
	Location string                    `json:"location"`
}

// GetPlacements returns IDs and locations of placement rules.
func (s *Service) GetPlacements(ctx context.Context) ([]PlacementInfo, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	placements := s.placement.All()
	infos := make([]PlacementInfo, 0, len(placements))
	for id, placement := range placements {
		infos = append(infos, PlacementInfo{
			ID:       id,
			Location: placement.Name,
		})
	}

	return infos, api.HTTPError{}
}
