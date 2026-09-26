// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package planaccess

import (
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// Exemption lets a user have a project on the given placement, even if the user
// doesn't meet the requirements (owned nodes, minimum number of active nodes,
// placement named after the owner). Exempt users also get their own placement
// enabled, regardless of the number of their nodes. An exemption for the
// default placement (0) lifts the lockdown of the default project.
//
// Exemptions are configured directly in YAML, without CLI flag support:
//
//	plan-access.exemptions:
//	  - user: 3d9a0b5e-4f3c-4e7a-9d8e-3b8a6c1f2e4d
//	    placement: 1
//	    limits:
//	      storage: 10TB
//	      rate-limit-put: 100
//	  - user: 3d9a0b5e4f3c4e7a9d8e3b8a6c1f2e4d
//	    placement: 0
//
// Users with multiple exempt placements are listed in multiple entries.
// Invalid entries are skipped with a warning, so a broken entry doesn't prevent
// the satellite from starting. That's also why the user ID is parsed later,
// and the placement is a pointer: a missing placement key must not be mistaken
// for the default placement.
type Exemption struct {
	User      string                     `yaml:"user"`
	Placement *storj.PlacementConstraint `yaml:"placement"`
	Limits    Limits                     `yaml:"limits"`

	// unknownKeys are the keys of the entry which are not known fields, to
	// warn about typos (which would silently drop the setting otherwise).
	unknownKeys []string
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (e *Exemption) UnmarshalYAML(value *yaml.Node) error {
	type plain Exemption
	var decoded plain
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	*e = Exemption(decoded)

	if value.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(value.Content); i += 2 {
			switch key := value.Content[i].Value; key {
			case "user", "placement", "limits":
			default:
				e.unknownKeys = append(e.unknownKeys, key)
			}
		}
	}
	return nil
}

// Limits are the project limits configured for an exempt placement. Nil values
// fall back to the limits the chore would set otherwise.
type Limits struct {
	Storage      *memory.Size `yaml:"storage"`
	Bandwidth    *memory.Size `yaml:"bandwidth"`
	Segment      *int64       `yaml:"segment"`
	RateLimitPut *int64       `yaml:"rate-limit-put"`
	RateLimitGet *int64       `yaml:"rate-limit-get"`
}

// exemptions are the limits of the exempt placements, by user.
type exemptions map[uuid.UUID]map[storj.PlacementConstraint]Limits

// newExemptions indexes the configured exemptions. Invalid entries are skipped
// with a warning.
func newExemptions(log *zap.Logger, entries []Exemption) exemptions {
	index := exemptions{}
	for i, entry := range entries {
		log := log.With(zap.Int("entry", i), zap.String("user", entry.User), zap.Strings("unknown_keys", entry.unknownKeys))

		if len(entry.unknownKeys) > 0 {
			log.Warn("exemption has unknown keys, they are ignored")
		}
		user, err := uuid.FromString(entry.User)
		if err != nil || user.IsZero() {
			log.Warn("skipping exemption with invalid user ID", zap.Error(err))
			continue
		}
		if entry.Placement == nil {
			log.Warn("skipping exemption without placement")
			continue
		}
		placement := *entry.Placement
		if _, ok := index[user][placement]; ok {
			log.Warn("skipping exemption, the placement is already listed for the user", zap.Uint16("placement", uint16(placement)))
			continue
		}
		if _, ok := index[user]; !ok {
			index[user] = map[storj.PlacementConstraint]Limits{}
		}
		index[user][placement] = entry.Limits
	}
	return index
}

// provisioned returns the limits of an exempt placement project: very high
// limits, unless configured otherwise.
func (limits Limits) provisioned() projectLimits {
	return limits.apply(projectLimits{
		storage:      int64Ptr(provisionedStorageLimit),
		bandwidth:    int64Ptr(provisionedBandwidthLimit),
		segment:      int64Ptr(enabledSegmentLimit),
		rateLimitPut: int64Ptr(provisionedRateLimit),
		rateLimitGet: int64Ptr(provisionedRateLimit),
	})
}

// apply overrides the given limits with the configured ones.
func (limits Limits) apply(to projectLimits) projectLimits {
	if limits.Storage != nil {
		to.storage = int64Ptr(limits.Storage.Int64())
	}
	if limits.Bandwidth != nil {
		to.bandwidth = int64Ptr(limits.Bandwidth.Int64())
	}
	if limits.Segment != nil {
		to.segment = limits.Segment
	}
	if limits.RateLimitPut != nil {
		to.rateLimitPut = limits.RateLimitPut
	}
	if limits.RateLimitGet != nil {
		to.rateLimitGet = limits.RateLimitGet
	}
	return to
}
