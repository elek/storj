// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/private/dbutil/pgutil"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

var mon = monkit.Package()

const defaultInterval = 15 * time.Second

// Peer represents one of StorageNode or Satellite.
type Peer interface {
	Label() string

	ID() storj.NodeID
	Addr() string
	URL() string
	NodeURL() storj.NodeURL

	Run(context.Context) error
	Close() error
}

// Config describes planet configuration.
type Config struct {
	SatelliteCount   int
	StorageNodeCount int
	UplinkCount      int
	MultinodeCount   int

	IdentityVersion *storj.IDVersion
	LastNetFunc     overlay.LastNetFunc
	Reconfigure     Reconfigure

	Name        string
	Host        string
	NonParallel bool
	Timeout     time.Duration

	applicationName string
}

// DatabaseConfig defines connection strings for database.
type DatabaseConfig struct {
	SatelliteDB string
}

// Planet is a full storj system setup.
type Planet struct {
	ctx       *testcontext.Context
	id        string
	log       *zap.Logger
	config    Config
	directory string // TODO: ensure that everything is in-memory to speed things up

	started  bool
	shutdown bool

	peers     []closablePeer
	databases []io.Closer
	uplinks   []*Uplink

	identities    *testidentity.Identities
	whitelistPath string // TODO: in-memory

	run    errgroup.Group
	cancel func()
}

type closablePeer struct {
	peer Peer

	ctx         context.Context
	cancel      func()
	runFinished chan struct{} // it is closed after peer.Run returns

	close sync.Once
	err   error
}

func newClosablePeer(peer Peer) closablePeer {
	return closablePeer{
		peer:        peer,
		runFinished: make(chan struct{}),
	}
}

// Close closes safely the peer.
func (peer *closablePeer) Close() error {
	peer.cancel()

	peer.close.Do(func() {
		<-peer.runFinished // wait for Run to complete
		peer.err = peer.peer.Close()
	})

	return peer.err
}

// NewCustom creates a new full system with the specified configuration.
func NewCustom(ctx *testcontext.Context, log *zap.Logger, config Config, satelliteDatabases satellitedbtest.SatelliteDatabases) (*Planet, error) {
	if config.IdentityVersion == nil {
		version := storj.LatestIDVersion()
		config.IdentityVersion = &version
	}

	if config.Host == "" {
		config.Host = "127.0.0.1"
		if hostlist := os.Getenv("STORJ_TEST_HOST"); hostlist != "" {
			hosts := strings.Split(hostlist, ";")
			config.Host = hosts[testrand.Intn(len(hosts))]
		}
	}

	if config.applicationName == "" {
		config.applicationName = "testplanet"
	}

	planet := &Planet{
		ctx:    ctx,
		log:    log,
		id:     config.Name + "/" + pgutil.CreateRandomTestingSchemaName(6),
		config: config,
	}

	if config.Reconfigure.Identities != nil {
		planet.identities = config.Reconfigure.Identities(log, *config.IdentityVersion)
	} else {
		planet.identities = testidentity.NewPregeneratedSignedIdentities(*config.IdentityVersion)
	}

	var err error
	planet.directory, err = os.MkdirTemp("", "planet")
	if err != nil {
		return nil, errs.Wrap(err)
	}

	whitelistPath, err := planet.WriteWhitelist(*config.IdentityVersion)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	planet.whitelistPath = whitelistPath

	err = planet.createPeers(ctx, satelliteDatabases)
	if err != nil {
		return nil, errs.Combine(err, planet.Shutdown())
	}
	return planet, nil
}
