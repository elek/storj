// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testgalaxy_test

import (
	"github.com/stretchr/testify/require"
	"storj.io/storj/private/testgalaxy"
	"storj.io/storj/private/testplanet"

	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
)

func TestBasic(t *testing.T) {
	for _, version := range storj.IDVersions {
		version := version
		testgalaxy.Run(t, testgalaxy.Config{
			SatelliteCount: 2, StorageNodeCount: 4, UplinkCount: 1,
			MultinodeCount: 1, IdentityVersion: &version,
		}, func(t *testing.T, ctx *testcontext.Context, planet *testgalaxy.Planet) {
			for _, satellite := range planet.Satellites {
				t.Log("SATELLITE", satellite.ID(), satellite.Addr())
			}
			for _, multitude := range planet.Multinodes {
				t.Log("MULTINODE", multitude.ID(), multitude.Addr())
			}
			for _, uplink := range planet.Uplinks {
				t.Log("UPLINK", uplink.ID(), uplink.Addr())
			}

			//for _, sat := range planet.Satellites {
			//	for _, sn := range planet.StorageNodes {
			//		func() {
			//			node := sn.Contact.Service.Local()
			//			conn, err := sn.Dialer.DialNodeURL(ctx, sat.NodeURL())
			//
			//			require.NoError(t, err)
			//			defer ctx.Check(conn.Close)
			//			_, err = pb.NewDRPCNodeClient(conn).CheckIn(ctx, &pb.CheckInRequest{
			//				Address:  node.Address,
			//				Version:  &node.Version,
			//				Capacity: &node.Capacity,
			//				Operator: &node.Operator,
			//			})
			//			require.NoError(t, err)
			//		}()
			//	},
			//}
			// wait a bit to see whether some failures occur
			time.Sleep(time.Second)
		})
	}
}

// test that nodes get put into each satellite's overlay cache.
func TestContact(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 5, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite0 := planet.Satellites[0]
		satellite1 := planet.Satellites[1]

		for _, n := range planet.StorageNodes {
			_, err := satellite0.Overlay.Service.Get(ctx, n.ID())
			require.NoError(t, err)
			_, err = satellite1.Overlay.Service.Get(ctx, n.ID())
			require.NoError(t, err)
		}
	})
}
