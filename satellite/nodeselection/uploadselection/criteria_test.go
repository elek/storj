// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package uploadselection

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/storj"
	"storj.io/common/testrand"
)

func TestCriteria_AutoExcludeSubnet(t *testing.T) {

	criteria := Criteria{
		AutoExcludeSubnets: map[string]struct{}{},
	}

	assert.True(t, criteria.MatchInclude(&Node{
		LastNet: "192.168.0.1",
	}))

	assert.False(t, criteria.MatchInclude(&Node{
		LastNet: "192.168.0.1",
	}))

	assert.True(t, criteria.MatchInclude(&Node{
		LastNet: "192.168.1.1",
	}))
}

func TestCriteria_ExcludeNodeID(t *testing.T) {
	included := testrand.NodeID()
	excluded := testrand.NodeID()

	criteria := Criteria{
		ExcludeNodeIDs: []storj.NodeID{excluded},
	}

	assert.False(t, criteria.MatchInclude(&Node{
		NodeURL: storj.NodeURL{
			ID:      excluded,
			Address: "localhost",
		},
	}))

	assert.True(t, criteria.MatchInclude(&Node{
		NodeURL: storj.NodeURL{
			ID:      included,
			Address: "localhost",
		},
	}))

}

func TestCriteria_NodeIDAndSubnet(t *testing.T) {
	excluded := testrand.NodeID()

	criteria := Criteria{
		ExcludeNodeIDs:     []storj.NodeID{excluded},
		AutoExcludeSubnets: map[string]struct{}{},
	}

	// due to node id criteria
	assert.False(t, criteria.MatchInclude(&Node{
		NodeURL: storj.NodeURL{
			ID:      excluded,
			Address: "192.168.0.1",
		},
	}))

	// should be included as previous one excluded and
	// not stored for subnet exclusion
	assert.True(t, criteria.MatchInclude(&Node{
		NodeURL: storj.NodeURL{
			ID:      testrand.NodeID(),
			Address: "192.168.0.2",
		},
	}))

}

func TestCriteria_Geofencing(t *testing.T) {
	excluded := testrand.NodeID()

	eu := Criteria{
		Placement: storj.EU,
	}

	us := Criteria{
		Placement: storj.US,
	}

	cases := []struct {
		name     string
		country  string
		criteria Criteria
		expected bool
	}{
		{
			name:     "US matches US selector",
			country:  "US",
			criteria: us,
			expected: true,
		},
		{
			name:     "Germany is EU",
			country:  "DE",
			criteria: eu,
			expected: true,
		},
		{
			name:     "US is not eu",
			country:  "US",
			criteria: eu,
			expected: false,
		},
		{
			name:     "Lower case country code is handled",
			country:  "de",
			criteria: eu,
			expected: true,
		},
		{
			name:     "Empty country doesn't match region",
			country:  "",
			criteria: eu,
			expected: false,
		},
		{
			name:     "Empty country doesn't match country",
			country:  "",
			criteria: us,
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, c.criteria.MatchInclude(&Node{
				NodeURL: storj.NodeURL{
					ID:      excluded,
					Address: "192.168.0.1",
				},
				CountryCode: c.country,
			}))
		})
	}
}
