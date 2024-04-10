// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

// See LICENSE for copying information.

package metabase

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/storj/private/mud/mudtest"
)

func TestBeginObjectSpanner(t *testing.T) {
	db := os.Getenv("STORJ_TEST_SPANNER")
	if db == "" {
		t.Skip("STORJ_TEST_SPANNER is not defined, no available spanner instance to test")
		return
	}

	mudtest.Run[*SpannerAdapter](t, SpannerTestModule,
		func(ctx context.Context, t *testing.T, adapter *SpannerAdapter) {
			uuid := testrand.UUID()
			o := &Object{}
			err := adapter.BeginObjectNextVersion(ctx, BeginObjectNextVersion{
				ObjectStream: ObjectStream{
					ProjectID: uuid,
				},
			}, o)
			require.NoError(t, err)
			require.Equal(t, Version(1), o.Version)

			err = adapter.BeginObjectNextVersion(ctx, BeginObjectNextVersion{
				ObjectStream: ObjectStream{
					ProjectID: uuid,
				},
			}, o)
			require.NoError(t, err)
			require.Equal(t, Version(2), o.Version)

		})

}
