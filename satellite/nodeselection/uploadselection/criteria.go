// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package uploadselection

import (
	"strings"

	"storj.io/common/storj"
)

// CountryCode is an uppercase ISO country code.
type CountryCode string

var eeaNonEuCountries = map[string]string{
	"IS": "Iceland",
	"LI": "Liechtenstein",
	"NO": "Norway",
}

var euCountries = map[string]string{
	"AT": "Austria",
	"BE": "Belgium",
	"BG": "Bulgaria",
	"CY": "Cyprus",
	"DE": "Germany",
	"DK": "Denmark",
	"EE": "Estonia",
	"ES": "Spain",
	"FI": "Finland",
	"FR": "France",
	"GR": "Greece",
	"HR": "Croatia",
	"HU": "Hungary",
	"IE": "Ireland",
	"IT": "Italy",
	"LT": "Lithuania",
	"LU": "Luxembourg",
	"LV": "Latvia",
	"MT": "Malta",
	"NL": "Netherlands",
	"PL": "Poland",
	"PT": "Portugal",
	"RO": "Romania",
	"RS": "Serbia",
	"SE": "Sweden",
	"SI": "Slovenia",
	"SK": "Slovakia",
}

// Criteria to filter nodes.
type Criteria struct {
	ExcludeNodeIDs     []storj.NodeID
	AutoExcludeSubnets map[string]struct{} // initialize it with empty map to keep only one node per subnet.
	Placement          storj.PlacementConstraint
}

// MatchInclude returns with true if node is selected.
func (c *Criteria) MatchInclude(node *Node) bool {
	if ContainsID(c.ExcludeNodeIDs, node.ID) {
		return false
	}

	if !allowedCountry(c.Placement, node.CountryCode) {
		return false
	}

	if c.AutoExcludeSubnets != nil {
		if _, excluded := c.AutoExcludeSubnets[node.LastNet]; excluded {
			return false
		}
		c.AutoExcludeSubnets[node.LastNet] = struct{}{}
	}
	return true
}

// ContainsID returns whether ids contain id.
func ContainsID(ids []storj.NodeID, id storj.NodeID) bool {
	for _, k := range ids {
		if k == id {
			return true
		}
	}
	return false
}

func allowedCountry(p storj.PlacementConstraint, isoCountryCode string) bool {
	if p == storj.EveryCountry {
		return true
	}
	countryCode := strings.ToUpper(isoCountryCode)
	switch p {
	case storj.EEA:
		if _, found := eeaNonEuCountries[countryCode]; found {
			return true
		}
		if _, found := euCountries[countryCode]; found {
			return true
		}
	case storj.EU:
		if _, found := euCountries[countryCode]; found {
			return true
		}
	case storj.US:
		return countryCode == "US"
	case storj.DE:
		return countryCode == "DE"
	default:
		return false
	}
	return false
}
