// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package analytics_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/shared/mud"
)

func TestModule(t *testing.T) {
	initball := func() *mud.Ball {
		ball := mud.NewBall()
		mud.Supply(ball, zaptest.NewLogger(t))
		analytics.Module(ball)

		// satellite.Module provides this, as it depends on consoleweb.Config.
		mud.Provide[*analytics.ReportingService](ball, func() *analytics.ReportingService {
			return analytics.NewReportingService(zaptest.NewLogger(t), analytics.Config{}, "test", "http://localhost")
		})
		return ball
	}

	t.Run("default implementation", func(t *testing.T) {
		ball := initball()

		ctx := testcontext.New(t)
		require.NoError(t, mud.ForEachDependency(ball, mud.All, mud.Initialize(ctx)))

		require.NoError(t, mud.Execute0(ctx, ball, func(service analytics.Service, freezeTracker analytics.FreezeTracker) {
			require.IsType(t, &analytics.ReportingService{}, service)
			require.IsType(t, &analytics.ReportingService{}, freezeTracker)
		}))
	})

	t.Run("replaced with noop", func(t *testing.T) {
		ball := initball()
		mud.ReplaceDependency[analytics.Service, *analytics.NoopService](ball)

		ctx := testcontext.New(t)
		require.NoError(t, mud.ForEachDependency(ball, mud.All, mud.Initialize(ctx)))

		require.NoError(t, mud.Execute0(ctx, ball, func(service analytics.Service, freezeTracker analytics.FreezeTracker) {
			require.IsType(t, &analytics.NoopService{}, service)
			// FreezeTracker follows the Service implementation.
			require.IsType(t, &analytics.NoopService{}, freezeTracker)
		}))
	})
}
