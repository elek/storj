// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package nodeinvites invites storage node operators to create a console account.
package nodeinvites

import (
	"context"
	"net/url"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/mailservice"
)

var (
	// Error is the standard error class for node invites.
	Error = errs.Class("node invites")
	mon   = monkit.Package()
)

// MailSender sends rendered emails. *mailservice.Service satisfies this interface.
type MailSender interface {
	SendRendered(ctx context.Context, to []post.Address, msg mailservice.Message) error
}

// Config holds configurable values for the node invites chore.
type Config struct {
	Interval     time.Duration `help:"how often to look for node operators to invite" default:"5m" devDefault:"30s"`
	MinNodes     int           `help:"minimum number of active nodes an operator must run to be invited" default:"20"`
	ActiveWindow time.Duration `help:"only count nodes that were contacted successfully within this window" default:"24h"`
	TokenTTL     time.Duration `help:"how long an invitation registration token is valid" default:"168h"`
	MaxAttempts  int           `help:"how many invitations to send to an operator before giving up" default:"3"`
	BatchSize    int           `help:"maximum number of invitations to send per interval" default:"10"`
}

// Candidate is a node operator that should be invited.
type Candidate struct {
	// Email is the lowercased operator email.
	Email string
	// NodeCount is the number of active nodes the operator runs.
	NodeCount int
}

// CandidateQuery holds the parameters for selecting invitation candidates.
type CandidateQuery struct {
	// MinNodes is the minimum number of active nodes an operator must run.
	MinNodes int
	// ActiveSince is the oldest successful contact that still counts as active.
	ActiveSince time.Time
	// Now is used to decide whether an existing registration token is still valid.
	Now time.Time
	// MaxAttempts is the number of invitations after which an operator is left alone.
	MaxAttempts int
	// Limit is the maximum number of candidates to return.
	Limit int
}

// DB selects the node operators that should be invited.
//
// architecture: Database
type DB interface {
	// GetCandidates returns node operators that qualify for an invitation.
	GetCandidates(ctx context.Context, query CandidateQuery) ([]Candidate, error)
}

// Chore invites storage node operators to create a console account.
//
// The invitation state lives in registration_tokens: tokens created here carry the
// operator email in the partner column, which is how the chore knows whether an
// operator already has an active invitation and how many they have received. The
// column is otherwise used for partner names, which never look like email
// addresses, so the two uses cannot collide.
type Chore struct {
	log           *zap.Logger
	db            DB
	tokens        console.RegistrationTokens
	mailService   MailSender
	consoleConfig consoleweb.Config
	config        Config
	nowFn         func() time.Time
	Loop          *sync2.Cycle
}

// NewChore creates a new Chore.
func NewChore(log *zap.Logger, db DB, tokens console.RegistrationTokens, mailService MailSender, consoleConfig consoleweb.Config, config Config) *Chore {
	return &Chore{
		log:           log,
		db:            db,
		tokens:        tokens,
		mailService:   mailService,
		consoleConfig: consoleConfig,
		config:        config,
		nowFn:         time.Now,
		Loop:          sync2.NewCycle(config.Interval),
	}
}

// Run runs the chore.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return chore.Loop.Run(ctx, func(ctx context.Context) error {
		if err := chore.RunOnce(ctx); err != nil {
			chore.log.Error("sending node operator invitations", zap.Error(err))
		}
		return nil
	})
}

// Close closes the chore.
func (chore *Chore) Close() error {
	chore.Loop.Close()
	return nil
}

// RunOnce sends invitations to a single batch of node operators.
func (chore *Chore) RunOnce(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := chore.nowFn()

	candidates, err := chore.db.GetCandidates(ctx, CandidateQuery{
		MinNodes:    chore.config.MinNodes,
		ActiveSince: now.Add(-chore.config.ActiveWindow),
		Now:         now,
		MaxAttempts: chore.config.MaxAttempts,
		Limit:       chore.config.BatchSize,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	mon.IntVal("node_invite_candidates").Observe(int64(len(candidates)))

	for i, candidate := range candidates {
		if err := chore.invite(ctx, candidate, now); err != nil {
			// Creating a token before sending the mail means a failed send still counts
			// as an attempt, so stop the batch instead of burning every operator's
			// attempts on the same outage.
			return Error.New("invited %d of %d operators: %w", i, len(candidates), err)
		}

		mon.Counter("node_invites_sent").Inc(1)
		chore.log.Info("invited node operator",
			zap.String("email", candidate.Email),
			zap.Int("node_count", candidate.NodeCount),
		)
	}

	return nil
}

// invite creates a registration token for the operator and emails them the signup link.
func (chore *Chore) invite(ctx context.Context, candidate Candidate, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	expiresAt := now.Add(chore.config.TokenTTL)
	kind := console.NFRUser
	var zeroLimit int64

	// N.B. the email must be stored lowercased: candidate selection matches it
	// against LOWER(nodes.email), and a mismatch would re-invite the operator on
	// every interval.
	token, err := chore.tokens.CreateWithLimits(ctx, console.CreateRegistrationTokenParams{
		ProjectLimit:   1,
		StorageLimit:   &zeroLimit,
		BandwidthLimit: &zeroLimit,
		SegmentLimit:   &zeroLimit,
		ExpiresAt:      &expiresAt,
		UserKind:       &kind,
		Partner:        &candidate.Email,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	link, err := chore.signupLink(token.Secret)
	if err != nil {
		return Error.Wrap(err)
	}

	err = chore.mailService.SendRendered(ctx,
		[]post.Address{{Address: candidate.Email}},
		&NodeOperatorInviteEmail{
			SignUpLink: link,
			NodeCount:  candidate.NodeCount,
		},
	)

	return Error.Wrap(err)
}

// signupLink returns the registration link for the given token secret.
func (chore *Chore) signupLink(secret console.RegistrationSecret) (string, error) {
	base, err := url.JoinPath(chore.consoleConfig.ExternalAddress, "signup")
	if err != nil {
		return "", err
	}

	return base + "?token=" + url.QueryEscape(secret.String()), nil
}

// TestSetNow allows tests to have the chore act as if the current time is whatever they want.
func (chore *Chore) TestSetNow(nowFn func() time.Time) {
	chore.nowFn = nowFn
}

// TestSetMailSender allows tests to replace the mail sender.
func (chore *Chore) TestSetMailSender(sender MailSender) {
	chore.mailService = sender
}
