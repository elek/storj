// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package planaccess gives node owners access to the capacity of their own
// nodes.
//
// Owners are the console users recorded in the owner tag of their confirmed
// nodes (see satellite/console/consoleext/nodes). Every owner gets a project on
// the default placement, locked down to zero limits. Owners also get a project
// on the placement named after their user ID, which is sized to the capacity of
// their nodes if they run enough active nodes, and locked down otherwise. The
// placement name is the owner tag value itself (see nodes.EncodeOwner), so that
// a placement filtering on the owner tag carries the same string as its name.
//
// Users listed in the exemptions file get their own placement enabled without
// meeting the requirements, and projects with very high (or configured) limits
// on the listed placements, even without owning any nodes.
package planaccess

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleext/nodes"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

var (
	// Error is the standard error class for plan access.
	Error = errs.Class("plan access")
	mon   = monkit.Package()
)

const (
	// ownPlacementRateLimitPut is the upload (put) rate limit of the project on
	// the owner's placement.
	ownPlacementRateLimitPut = 10000
	// ownPlacementRateLimitGet is the download (get) rate limit of the project on
	// the owner's placement.
	ownPlacementRateLimitGet = 10000
	// capacityRatio is the share of the owner's node capacity (free + used) that
	// is allowed to be stored in the project on the owner's placement.
	capacityRatio = 0.95

	// provisionedStorageLimit is the storage limit of exempt placement projects,
	// unless configured otherwise.
	provisionedStorageLimit = int64(memory.PB)
	// provisionedBandwidthLimit is the bandwidth limit of exempt placement
	// projects, unless configured otherwise.
	provisionedBandwidthLimit = int64(memory.PB)
	// enabledSegmentLimit is the segment limit of enabled projects. Setting it
	// also lifts the zero segment limit of a previously locked down project.
	enabledSegmentLimit = 1_000_000_000
	// provisionedRateLimit is the put and get rate limit of exempt placement
	// projects, unless configured otherwise.
	provisionedRateLimit = 10000

	defaultProjectName = "default"
	ownProjectName     = "own nodes"

	defaultProjectDescription     = "Disabled due to not enough SNOs"
	ownProjectDisabledDescription = "Disabled due to not enough nodes"
	ownProjectEnabledDescription  = "Stored on own storagenodes"
	provisionedProjectDescription = "Provisioned to placement %s"
	maxProjectDescriptionLength   = 100
)

// Config holds configurable values for the plan access chore.
type Config struct {
	Interval      time.Duration `help:"how often to reconcile the projects of node owners" default:"1h" devDefault:"1m"`
	OnlineWindow  time.Duration `help:"only nodes contacted within this window count towards the capacity of their owner" default:"4h"`
	TallyLookback time.Duration `help:"how far back to look for node tallies when estimating the used space of nodes" default:"48h"`
	MinNodes      int           `help:"minimum number of active owned nodes required to set up the project on the owner's placement" default:"20"`
	Exemptions    []Exemption   `noflag:"true"`
}

// Chore makes sure that the owners of confirmed nodes have projects with the
// right placements and limits.
type Chore struct {
	log         *zap.Logger
	overlayDB   overlay.DB
	consoleDB   console.DB
	accounting  accounting.StoragenodeAccounting
	placements  nodeselection.PlacementProvider
	satelliteID storj.NodeID
	exemptions  exemptions
	config      Config
	Loop        *sync2.Cycle
}

// NewChore creates a new Chore.
func NewChore(log *zap.Logger, overlayDB overlay.DB, consoleDB console.DB, accountingDB accounting.StoragenodeAccounting, placements nodeselection.PlacementProvider, satelliteID storj.NodeID, config Config) *Chore {
	exemptions := newExemptions(log, config.Exemptions)
	if len(exemptions) > 0 {
		log.Info("loaded project provisioning exemptions", zap.Int("users", len(exemptions)))
	}
	return &Chore{
		log:         log,
		overlayDB:   overlayDB,
		consoleDB:   consoleDB,
		accounting:  accountingDB,
		placements:  placements,
		satelliteID: satelliteID,
		exemptions:  exemptions,
		config:      config,
		Loop:        sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		if err := chore.RunOnce(ctx); err != nil {
			chore.log.Error("reconciling projects of node owners", zap.Error(err))
		}
		return nil
	})
}

// Close closes the chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}

// ownerGroup is the set of nodes confirmed by a single owner.
type ownerGroup struct {
	owner uuid.UUID
	nodes []*nodeselection.SelectedNode
}

// RunOnce reconciles the projects of every node owner once.
func (chore *Chore) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	groups, err := chore.ownerGroups(ctx)
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		return nil
	}

	used, err := chore.usedSpace(ctx)
	if err != nil {
		return err
	}

	var group errs.Group
	for _, g := range groups {
		if err := chore.reconcile(ctx, g, used); err != nil {
			// one broken owner shouldn't stop the others from being reconciled.
			chore.log.Error("reconciling projects of node owner", zap.Stringer("owner", g.owner), zap.Error(err))
			group.Add(err)
		}
	}
	return group.Err()
}

// ownerGroups groups the participating nodes by the owner recorded in the owner
// tag signed by this satellite. Nodes without such a tag are left out. Exempt
// users are included even without any nodes.
func (chore *Chore) ownerGroups(ctx context.Context) (_ []ownerGroup, err error) {
	defer mon.Task()(&ctx)(&err)

	selected, err := chore.overlayDB.SelectAllStorageNodesDownload(ctx, chore.config.OnlineWindow, overlay.AsOfSystemTimeConfig{})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	byOwner := map[uuid.UUID]*ownerGroup{}
	for _, node := range selected {
		tag, err := node.Tags.FindBySignerAndName(chore.satelliteID, nodes.OwnerTagName)
		if err != nil {
			continue
		}
		owner, err := nodes.DecodeOwner(tag.Value)
		if err != nil || owner.IsZero() {
			chore.log.Warn("invalid owner tag", zap.Stringer("node", node.ID))
			continue
		}
		g, ok := byOwner[owner]
		if !ok {
			g = &ownerGroup{owner: owner}
			byOwner[owner] = g
		}
		g.nodes = append(g.nodes, node)
	}
	for owner := range chore.exemptions {
		if _, ok := byOwner[owner]; !ok {
			byOwner[owner] = &ownerGroup{owner: owner}
		}
	}

	groups := make([]ownerGroup, 0, len(byOwner))
	for _, g := range byOwner {
		groups = append(groups, *g)
	}
	// a stable order keeps the logs of consecutive runs comparable.
	sort.Slice(groups, func(i, j int) bool { return groups[i].owner.Less(groups[j].owner) })
	return groups, nil
}

// reconcile ensures the projects and limits of a single owner.
func (chore *Chore) reconcile(ctx context.Context, g ownerGroup, used map[storj.NodeID]int64) (err error) {
	defer mon.Task()(&ctx)(&err)

	log := chore.log.With(zap.Stringer("owner", g.owner))

	user, err := chore.consoleDB.Users().Get(ctx, g.owner)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("owner of confirmed nodes does not exist")
			return nil
		}
		return Error.Wrap(err)
	}
	if user.Status != console.Active {
		log.Debug("owner of confirmed nodes is not active", zap.Int("status", int(user.Status)))
		return nil
	}

	projects, err := chore.consoleDB.Projects().GetOwnActive(ctx, user.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	exempt, isExempt := chore.exemptions[user.ID]

	if limits, ok := exempt[storj.DefaultPlacement]; ok {
		defaultProject, err := chore.ensureProject(ctx, user, &projects, storj.DefaultPlacement, defaultProjectName,
			fmt.Sprintf(provisionedProjectDescription, chore.placementLabel(storj.DefaultPlacement)))
		if err != nil {
			return err
		}
		if err := chore.updateLimits(ctx, defaultProject, limits.provisioned()); err != nil {
			return err
		}
	} else {
		defaultProject, err := chore.ensureProject(ctx, user, &projects, storj.DefaultPlacement, defaultProjectName, defaultProjectDescription)
		if err != nil {
			return err
		}
		if err := chore.updateLimits(ctx, defaultProject, lockedDown()); err != nil {
			return err
		}
	}

	// N.B. the placement name has to match the owner tag value, not the dashed
	// form of the user ID.
	placementName := string(nodes.EncodeOwner(user.ID))
	ownPlacement, hasOwnPlacement := nodeselection.FindPlacementByName(chore.placements, placementName)
	if hasOwnPlacement {
		if err := chore.reconcileOwnPlacement(ctx, log, user, &projects, ownPlacement, g.nodes, used, exempt, isExempt); err != nil {
			return err
		}
	} else if len(g.nodes) > 0 {
		log.Warn("no placement is defined for the owner of confirmed nodes", zap.String("placement_name", placementName))
	}

	placements := make([]storj.PlacementConstraint, 0, len(exempt))
	for placement := range exempt {
		placements = append(placements, placement)
	}
	slices.Sort(placements)

	for _, placement := range placements {
		// the default and the own placement projects are handled above.
		if placement == storj.DefaultPlacement || (hasOwnPlacement && placement == ownPlacement) {
			continue
		}
		// placements may be removed from the placement config after the
		// exemptions are loaded.
		if _, found := chore.placements.Get(placement); !found {
			log.Warn("exempt placement of the owner is not defined", zap.Uint16("placement", uint16(placement)))
			continue
		}

		project, err := chore.ensureProject(ctx, user, &projects, placement,
			fmt.Sprintf("placement %d", placement), fmt.Sprintf(provisionedProjectDescription, chore.placementLabel(placement)))
		if err != nil {
			return err
		}
		if err := chore.updateLimits(ctx, project, exempt[placement].provisioned()); err != nil {
			return err
		}
	}
	return nil
}

// placementLabel returns the name of the placement, or its ID if it has no name.
func (chore *Chore) placementLabel(placement storj.PlacementConstraint) string {
	if definition, found := chore.placements.Get(placement); found && definition.Name != "" {
		return definition.Name
	}
	return fmt.Sprint(uint16(placement))
}

// reconcileOwnPlacement ensures the project on the placement named after the
// owner. It's sized to the capacity of the owner's nodes if the owner runs
// enough active nodes or is exempt, and locked down otherwise.
func (chore *Chore) reconcileOwnPlacement(ctx context.Context, log *zap.Logger, user *console.User, projects *[]console.Project, placement storj.PlacementConstraint, owned []*nodeselection.SelectedNode, used map[storj.NodeID]int64, exempt map[storj.PlacementConstraint]Limits, isExempt bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	active := activeNodes(owned)
	if active < chore.config.MinNodes && !isExempt {
		log.Debug("not enough active owned nodes for the owner's placement",
			zap.Int("active_nodes", active), zap.Int("min_nodes", chore.config.MinNodes))

		project, err := chore.ensureProject(ctx, user, projects, placement, ownProjectName, ownProjectDisabledDescription)
		if err != nil {
			return err
		}
		return chore.updateLimits(ctx, project, lockedDown())
	}

	project, err := chore.ensureProject(ctx, user, projects, placement, ownProjectName, ownProjectEnabledDescription)
	if err != nil {
		return err
	}

	var capacity int64
	for _, node := range owned {
		// free_disk is -1 until the node reports it.
		capacity += max(node.FreeDisk, 0) + used[node.ID]
	}
	return chore.updateLimits(ctx, project, exempt[placement].apply(projectLimits{
		storage:      int64Ptr(int64(float64(capacity) * capacityRatio)),
		segment:      int64Ptr(enabledSegmentLimit),
		rateLimitPut: int64Ptr(ownPlacementRateLimitPut),
		rateLimitGet: int64Ptr(ownPlacementRateLimitGet),
	}))
}

// lockedDown returns the limits which disable uploads and downloads.
func lockedDown() projectLimits {
	return projectLimits{
		rateLimitPut: int64Ptr(0),
		rateLimitGet: int64Ptr(0),
		segment:      int64Ptr(0),
	}
}

// ensureProject returns the oldest project of the user with the given default
// placement, creating one if there is none, and sets its description. Created
// projects are appended to projects, so that later calls see them.
func (chore *Chore) ensureProject(ctx context.Context, user *console.User, projects *[]console.Project, placement storj.PlacementConstraint, name, description string) (_ *console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(description) > maxProjectDescriptionLength {
		description = description[:maxProjectDescriptionLength]
	}

	var found *console.Project
	names := map[string]bool{}
	for i := range *projects {
		p := &(*projects)[i]
		names[p.Name] = true
		if p.DefaultPlacement == placement && (found == nil || p.CreatedAt.Before(found.CreatedAt)) {
			found = p
		}
	}
	if found != nil {
		if found.Description != description {
			// N.B. Update writes the other fields too, so it has to happen
			// before updating the limits of the project.
			found.Description = description
			if err := chore.consoleDB.Projects().Update(ctx, found); err != nil {
				return nil, Error.Wrap(err)
			}
		}
		return found, nil
	}

	// project names are unique per owner.
	uniqueName := name
	for i := 2; names[uniqueName]; i++ {
		uniqueName = fmt.Sprintf("%s %d", name, i)
	}

	var created *console.Project
	err = chore.consoleDB.WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
		var err error
		created, err = tx.Projects().Insert(ctx, &console.Project{
			Name:             uniqueName,
			OwnerID:          user.ID,
			UserAgent:        user.UserAgent,
			Description:      description,
			DefaultPlacement: placement,
		})
		if err != nil {
			return err
		}
		_, err = tx.ProjectMembers().Insert(ctx, user.ID, created.ID, console.RoleAdmin)
		return err
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}
	*projects = append(*projects, *created)

	chore.log.Info("created project for node owner",
		zap.Stringer("owner", user.ID),
		zap.Stringer("project", created.ID),
		zap.Uint16("placement", uint16(placement)))
	return created, nil
}

// projectLimits are the limits the chore sets on a project. Nil values are
// left as they are.
type projectLimits struct {
	storage      *int64
	bandwidth    *int64
	segment      *int64
	rateLimitPut *int64
	rateLimitGet *int64
}

// updateLimits sets the limits of the project which differ from the wanted ones.
func (chore *Chore) updateLimits(ctx context.Context, project *console.Project, want projectLimits) (err error) {
	defer mon.Task()(&ctx)(&err)

	var toUpdate []console.Limit
	add := func(kind console.LimitKind, current, wanted *int64) {
		if wanted == nil || (current != nil && *current == *wanted) {
			return
		}
		toUpdate = append(toUpdate, console.Limit{Kind: kind, Value: wanted})
	}

	var storage *int64
	if project.StorageLimit != nil {
		v := project.StorageLimit.Int64()
		storage = &v
	}
	add(console.StorageLimit, storage, want.storage)
	var bandwidth *int64
	if project.BandwidthLimit != nil {
		v := project.BandwidthLimit.Int64()
		bandwidth = &v
	}
	add(console.BandwidthLimit, bandwidth, want.bandwidth)
	add(console.SegmentLimit, project.SegmentLimit, want.segment)
	add(console.RateLimitPut, intToInt64(project.RateLimitPut), want.rateLimitPut)
	add(console.RateLimitGet, intToInt64(project.RateLimitGet), want.rateLimitGet)

	if len(toUpdate) == 0 {
		return nil
	}

	err = chore.consoleDB.Projects().UpdateLimitsGeneric(ctx, project.ID, toUpdate)
	if err != nil {
		return Error.Wrap(err)
	}

	fields := []zap.Field{zap.Stringer("owner", project.OwnerID), zap.Stringer("project", project.ID)}
	for _, limit := range toUpdate {
		fields = append(fields, zap.Int64(limitName(limit.Kind), *limit.Value))
	}
	chore.log.Info("updated limits of node owner project", fields...)
	return nil
}

// usedSpace estimates the bytes stored on each node from the latest node
// tally. Tallies are saved in byte-hours, accumulated since the previous
// tally, so the latest one is divided by the hours between the two.
//
// Nodes without a recent tally are missing from the result.
func (chore *Chore) usedSpace(ctx context.Context) (_ map[storj.NodeID]int64, err error) {
	defer mon.Task()(&ctx)(&err)

	latest, err := chore.accounting.LastTimestamp(ctx, accounting.LastAtRestTally)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	if latest.IsZero() {
		chore.log.Warn("there are no node tallies yet, used space of the nodes is not counted")
		return nil, nil
	}

	tallies, err := chore.accounting.GetTalliesSince(ctx, latest.Add(-chore.config.TallyLookback))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// the observer saves all tallies of a run with the same interval end time,
	// so the previous run is the latest interval end time before the last one.
	var previous time.Time
	for _, tally := range tallies {
		if tally.IntervalEndTime.Before(latest) && tally.IntervalEndTime.After(previous) {
			previous = tally.IntervalEndTime
		}
	}
	if previous.IsZero() {
		chore.log.Warn("there is no previous node tally within the lookback, used space of the nodes is not counted",
			zap.Time("last_tally", latest), zap.Duration("lookback", chore.config.TallyLookback))
		return nil, nil
	}
	hours := latest.Sub(previous).Hours()

	used := map[storj.NodeID]int64{}
	for _, tally := range tallies {
		if tally.IntervalEndTime.Equal(latest) {
			used[tally.NodeID] += int64(tally.DataTotal / hours)
		}
	}
	return used, nil
}

// activeNodes counts the nodes which are neither suspended nor exiting. The
// nodes are already known to be online, not disqualified and not exited.
func activeNodes(nodes []*nodeselection.SelectedNode) (count int) {
	for _, node := range nodes {
		if !node.Suspended && !node.Exiting {
			count++
		}
	}
	return count
}

func int64Ptr(v int64) *int64 { return &v }

func intToInt64(v *int) *int64 {
	if v == nil {
		return nil
	}
	v64 := int64(*v)
	return &v64
}

func limitName(kind console.LimitKind) string {
	switch kind {
	case console.StorageLimit:
		return "storage"
	case console.BandwidthLimit:
		return "bandwidth"
	case console.SegmentLimit:
		return "segment"
	case console.RateLimitPut:
		return "rate_limit_put"
	case console.RateLimitGet:
		return "rate_limit_get"
	default:
		return fmt.Sprintf("limit_%d", kind)
	}
}
