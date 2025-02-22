// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"encoding/json"
	"time"

	"github.com/spf13/pflag"

	"storj.io/common/storj"
)

// Config keeps track of core console service configuration parameters.
type Config struct {
	PasswordCost                    int                       `help:"password hashing cost (0=automatic)" testDefault:"4" default:"0"`
	OpenRegistrationEnabled         bool                      `help:"enable open registration" default:"false" testDefault:"true"`
	DefaultProjectLimit             int                       `help:"default project limits for users" default:"1" testDefault:"5"`
	AsOfSystemTimeDuration          time.Duration             `help:"default duration for AS OF SYSTEM TIME" devDefault:"-5m" releaseDefault:"-5m" testDefault:"0"`
	LoginAttemptsWithoutPenalty     int                       `help:"number of times user can try to login without penalty" default:"3"`
	FailedLoginPenalty              float64                   `help:"incremental duration of penalty for failed login attempts in minutes" default:"2.0"`
	ProjectInvitationExpiration     time.Duration             `help:"duration that project member invitations are valid for" default:"168h"`
	UnregisteredInviteEmailsEnabled bool                      `help:"indicates whether invitation emails can be sent to unregistered email addresses" default:"false"`
	FreeTierInvitesEnabled          bool                      `help:"indicates whether free tier users can send project invitations" default:"false"`
	UserBalanceForUpgrade           int64                     `help:"amount of base units of US micro dollars needed to upgrade user's tier status" default:"10000000"`
	PlacementEdgeURLOverrides       PlacementEdgeURLOverrides `help:"placement-specific edge service URL overrides in the format {\"placementID\": {\"authService\": \"...\", \"publicLinksharing\": \"...\", \"internalLinksharing\": \"...\"}, \"placementID2\": ...}"`
	BlockExplorerURL                string                    `help:"url of the transaction block explorer" default:"https://etherscan.io/"`
	BillingFeaturesEnabled          bool                      `help:"indicates if billing features should be enabled" default:"true"`
	StripePaymentElementEnabled     bool                      `help:"indicates whether the stripe payment element should be used to collect card info" default:"true"`
	SignupActivationCodeEnabled     bool                      `help:"indicates whether the whether account activation is done using activation code" default:"false"`
	UsageLimits                     UsageLimitsConfig
	Captcha                         CaptchaConfig
	Session                         SessionConfig
	AccountFreeze                   AccountFreezeConfig
}

// CaptchaConfig contains configurations for login/registration captcha system.
type CaptchaConfig struct {
	Login        MultiCaptchaConfig `json:"login"`
	Registration MultiCaptchaConfig `json:"registration"`
}

// MultiCaptchaConfig contains configurations for Recaptcha and Hcaptcha systems.
type MultiCaptchaConfig struct {
	Recaptcha SingleCaptchaConfig `json:"recaptcha"`
	Hcaptcha  SingleCaptchaConfig `json:"hcaptcha"`
}

// SingleCaptchaConfig contains configurations abstract captcha system.
type SingleCaptchaConfig struct {
	Enabled   bool   `help:"whether or not captcha is enabled" default:"false" json:"enabled"`
	SiteKey   string `help:"captcha site key" json:"siteKey"`
	SecretKey string `help:"captcha secret key" json:"-"`
}

// SessionConfig contains configurations for session management.
type SessionConfig struct {
	InactivityTimerEnabled       bool          `help:"indicates if session can be timed out due inactivity" default:"true"`
	InactivityTimerDuration      int           `help:"inactivity timer delay in seconds" default:"600"`
	InactivityTimerViewerEnabled bool          `help:"indicates whether remaining session time is shown for debugging" default:"false"`
	Duration                     time.Duration `help:"duration a session is valid for (superseded by inactivity timer delay if inactivity timer is enabled)" default:"168h"`
}

// EdgeURLOverrides contains edge service URL overrides.
type EdgeURLOverrides struct {
	AuthService         string `json:"authService,omitempty"`
	PublicLinksharing   string `json:"publicLinksharing,omitempty"`
	InternalLinksharing string `json:"internalLinksharing,omitempty"`
}

// PlacementEdgeURLOverrides represents a mapping between placement IDs and edge service URL overrides.
type PlacementEdgeURLOverrides struct {
	overrideMap map[storj.PlacementConstraint]EdgeURLOverrides
}

// Ensure that PlacementEdgeOverrides implements pflag.Value.
var _ pflag.Value = (*PlacementEdgeURLOverrides)(nil)

// Type implements pflag.Value.
func (PlacementEdgeURLOverrides) Type() string { return "console.PlacementEdgeURLOverrides" }

// String implements pflag.Value.
func (ov *PlacementEdgeURLOverrides) String() string {
	if ov == nil || len(ov.overrideMap) == 0 {
		return ""
	}

	overrides, err := json.Marshal(ov.overrideMap)
	if err != nil {
		return ""
	}

	return string(overrides)
}

// Set implements pflag.Value.
func (ov *PlacementEdgeURLOverrides) Set(s string) error {
	if s == "" {
		return nil
	}

	overrides := make(map[storj.PlacementConstraint]EdgeURLOverrides)
	err := json.Unmarshal([]byte(s), &overrides)
	if err != nil {
		return err
	}
	ov.overrideMap = overrides

	return nil
}

// Get returns the edge service URL overrides for the given placement ID.
func (ov *PlacementEdgeURLOverrides) Get(placement storj.PlacementConstraint) (overrides EdgeURLOverrides, ok bool) {
	if ov == nil {
		return EdgeURLOverrides{}, false
	}
	overrides, ok = ov.overrideMap[placement]
	return overrides, ok
}
