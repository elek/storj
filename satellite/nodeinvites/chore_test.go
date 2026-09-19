// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeinvites_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/post"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/nodeinvites"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/mudplanet"
	"storj.io/storj/shared/mudplanet/satellitetest"
)

const externalAddress = "https://satellite.test/"

type captureSender struct {
	mu   sync.Mutex
	sent []mailservice.Message
	to   []string
	err  error
}

func (s *captureSender) SendRendered(_ context.Context, to []post.Address, msg mailservice.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	for _, address := range to {
		s.to = append(s.to, address.Address)
	}
	s.sent = append(s.sent, msg)
	return nil
}

func (s *captureSender) invites() []*nodeinvites.NodeOperatorInviteEmail {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*nodeinvites.NodeOperatorInviteEmail, 0, len(s.sent))
	for _, msg := range s.sent {
		invite, ok := msg.(*nodeinvites.NodeOperatorInviteEmail)
		if !ok {
			continue
		}
		out = append(out, invite)
	}
	return out
}

func (s *captureSender) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = nil
	s.to = nil
	s.err = nil
}

// addNodes checks in count nodes reporting the given operator email at lastContact.
func addNodes(ctx context.Context, t *testing.T, cache overlay.DB, email string, count int, lastContact time.Time) {
	t.Helper()
	for i := 0; i < count; i++ {
		require.NoError(t, cache.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
			NodeID:  testrand.NodeID(),
			IsUp:    true,
			Address: &pb.NodeAddress{Address: "1.2.3.4"},
			Version: &pb.NodeVersion{Version: "v0.0.0"},
			Operator: &pb.NodeOperator{
				Email:  email,
				Wallet: "0x1234567890123456789012345678901234567890",
			},
		}, lastContact, overlay.NodeSelectionConfig{OnlineWindow: 4 * time.Hour}))
	}
}

// secretFromLink extracts the registration token secret from a signup link.
func secretFromLink(t *testing.T, link string) console.RegistrationSecret {
	t.Helper()
	parsed, err := url.Parse(link)
	require.NoError(t, err)
	secret, err := console.RegistrationSecretFromBase64(parsed.Query().Get("token"))
	require.NoError(t, err)
	return secret
}

func newChore(t *testing.T, db satellite.DB, sender nodeinvites.MailSender, config nodeinvites.Config) *nodeinvites.Chore {
	return nodeinvites.NewChore(
		zaptest.NewLogger(t),
		db.NodeInvites(),
		db.Console().RegistrationTokens(),
		sender,
		consoleweb.Config{
			Config: console.Config{ExternalAddress: externalAddress},
		},
		config,
	)
}

func testConfig() nodeinvites.Config {
	return nodeinvites.Config{
		Interval:     time.Hour,
		MinNodes:     3,
		ActiveWindow: 24 * time.Hour,
		TokenTTL:     168 * time.Hour,
		MaxAttempts:  3,
		BatchSize:    10,
	}
}

func TestChoreInvites(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()
		tokens := db.Console().RegistrationTokens()

		sender := &captureSender{}
		chore := newChore(t, db, sender, testConfig())

		now := time.Now()
		chore.TestSetNow(func() time.Time { return now })

		// the operator reports mixed casing at check-in.
		addNodes(ctx, t, cache, "Operator@Storj.Test", 4, now)
		addNodes(ctx, t, cache, "toofew@storj.test", 2, now)

		require.NoError(t, chore.RunOnce(ctx))

		invites := sender.invites()
		require.Len(t, invites, 1)
		require.Equal(t, []string{"operator@storj.test"}, sender.to)
		require.Equal(t, 4, invites[0].NodeCount)
		require.True(t, strings.HasPrefix(invites[0].SignUpLink, externalAddress+"signup?token="),
			"unexpected signup link %q", invites[0].SignUpLink)

		token, err := tokens.GetBySecret(ctx, secretFromLink(t, invites[0].SignUpLink))
		require.NoError(t, err)

		// the email is stored lowercased, otherwise the operator would be invited again
		// on the next interval.
		require.NotNil(t, token.Partner)
		require.Equal(t, "operator@storj.test", *token.Partner)

		require.Equal(t, 1, token.ProjectLimit)
		require.NotNil(t, token.StorageLimit)
		require.Zero(t, *token.StorageLimit)
		require.NotNil(t, token.BandwidthLimit)
		require.Zero(t, *token.BandwidthLimit)
		require.NotNil(t, token.SegmentLimit)
		require.Zero(t, *token.SegmentLimit)
		require.NotNil(t, token.UserKind)
		require.Equal(t, console.NFRUser, *token.UserKind)
		require.NotNil(t, token.ExpiresAt)
		require.WithinDuration(t, now.Add(168*time.Hour), *token.ExpiresAt, time.Minute)
		require.Nil(t, token.OwnerID)

		// the active token stops the operator from being invited again.
		sender.reset()
		require.NoError(t, chore.RunOnce(ctx))
		require.Empty(t, sender.invites())

		// once the token expired the operator is invited again. the nodes need a recent
		// contact relative to the shifted clock too.
		sender.reset()
		later := now.Add(169 * time.Hour)
		addNodes(ctx, t, cache, "Operator@Storj.Test", 4, later)
		chore.TestSetNow(func() time.Time { return later })
		require.NoError(t, chore.RunOnce(ctx))
		require.Len(t, sender.invites(), 1)
	})
}

func TestChoreBatchSize(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		config := testConfig()
		config.BatchSize = 2

		sender := &captureSender{}
		chore := newChore(t, db, sender, config)

		addNodes(ctx, t, cache, "one@storj.test", 5, time.Now())
		addNodes(ctx, t, cache, "two@storj.test", 4, time.Now())
		addNodes(ctx, t, cache, "three@storj.test", 3, time.Now())

		require.NoError(t, chore.RunOnce(ctx))

		// the operators running the most nodes are invited first.
		require.Equal(t, []string{"one@storj.test", "two@storj.test"}, sender.to)

		require.NoError(t, chore.RunOnce(ctx))
		require.Equal(t, []string{"one@storj.test", "two@storj.test", "three@storj.test"}, sender.to)
	})
}

func TestChoreStopsBatchOnSendError(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		cache := db.OverlayCache()

		sender := &captureSender{err: errors.New("smtp is down")}
		chore := newChore(t, db, sender, testConfig())

		addNodes(ctx, t, cache, "one@storj.test", 5, time.Now())
		addNodes(ctx, t, cache, "two@storj.test", 4, time.Now())
		addNodes(ctx, t, cache, "three@storj.test", 3, time.Now())

		require.Error(t, chore.RunOnce(ctx))
		require.Empty(t, sender.invites())

		// only the first operator's attempt was consumed, the rest are untouched.
		sender.reset()
		require.NoError(t, chore.RunOnce(ctx))
		require.Equal(t, []string{"two@storj.test", "three@storj.test"}, sender.to)
	})
}

// TestChoreInvitedOperatorRegisters checks that the issued token creates the account the
// chore promises: one project, no storage, and no trial expiration.
func TestChoreInvitedOperatorRegisters(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		sender := &captureSender{}
		chore := newChore(t, sat.DB, sender, testConfig())

		addNodes(ctx, t, sat.DB.OverlayCache(), "operator@storj.test", 3, time.Now())

		require.NoError(t, chore.RunOnce(ctx))
		invites := sender.invites()
		require.Len(t, invites, 1)

		// register the way an invited operator does: the secret from the link, submitted
		// to the registration endpoint. The user kind comes from the token there, not
		// from console.Service.CreateUser.
		secret := secretFromLink(t, invites[0].SignUpLink)
		body, err := json.Marshal(map[string]string{
			"fullName": "Node Operator",
			"email":    "operator@storj.test",
			"password": "password1234",
			"secret":   secret.String(),
		})
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			sat.ConsoleURL()+"/api/v0/auth/register", bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		result, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, result.Body.Close())
		require.Equal(t, http.StatusOK, result.StatusCode)

		_, unverified, err := sat.API.Console.Service.GetUserByEmailWithUnverified(ctx, "operator@storj.test")
		require.NoError(t, err)
		require.Len(t, unverified, 1)
		user := unverified[0]

		require.Equal(t, console.NFRUser, user.Kind)
		require.Equal(t, 1, user.ProjectLimit)
		require.Zero(t, user.ProjectStorageLimit)
		require.Zero(t, user.ProjectBandwidthLimit)
		require.Zero(t, user.ProjectSegmentLimit)
		// NFR accounts don't expire like a trial.
		require.Nil(t, user.TrialExpiration)

		// the token is spent, so the operator is never invited again.
		sender.reset()
		require.NoError(t, chore.RunOnce(ctx))
		require.Empty(t, sender.invites())
	})
}

// TestChoreMudWiring checks that the chore can be built and started from the mud graph,
// which is the only way it is wired up.
func TestChoreMudWiring(t *testing.T) {
	mudplanet.Run(t, satellitetest.WithDB(
		mudplanet.NewComponent("satellite", satellitetest.Satellite,
			mudplanet.WithRunning[*nodeinvites.Chore](),
			mudplanet.WithConfig(func(cfg *mailservice.Config) {
				cfg.TemplatePath = "../../web/satellite/static/emails"
			}),
		),
	), func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		chore := mudplanet.FindFirst[*nodeinvites.Chore](t, run, "satellite", 0)
		require.NotNil(t, chore)
	})
}
