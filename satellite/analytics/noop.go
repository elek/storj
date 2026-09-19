// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package analytics

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// NoopService is a Service implementation which drops all the events.
//
// architecture: Service
type NoopService struct {
}

// NewNoopService creates a new Service which doesn't report anything.
func NewNoopService() *NoopService {
	return &NoopService{}
}

// Run implements Service.
func (n *NoopService) Run(ctx context.Context) error { return nil }

// Close implements Service.
func (n *NoopService) Close() error { return nil }

// TrackCreateUser implements Service.
func (n *NoopService) TrackCreateUser(fields TrackCreateUserFields) {}

// TrackDeleteUser implements Service.
func (n *NoopService) TrackDeleteUser(userID uuid.UUID, email string, adminInitiated bool, hubspotObjectID, tenantID *string) {
}

// JoinPlacementWaitlist implements Service.
func (n *NoopService) JoinPlacementWaitlist(fields TrackJoinPlacementWaitlistFields) {}

// ChangeContactEmail implements Service.
func (n *NoopService) ChangeContactEmail(userID uuid.UUID, oldEmail, newEmail string) {}

// TrackUserOnboardingInfo implements Service.
func (n *NoopService) TrackUserOnboardingInfo(fields TrackOnboardingInfoFields) {}

// TrackSignedIn implements Service.
func (n *NoopService) TrackSignedIn(userID uuid.UUID, email, anonymousID string, hubspotObjectID, tenantID *string) {
}

// TrackProjectCreated implements Service.
func (n *NoopService) TrackProjectCreated(userID uuid.UUID, email string, projectID uuid.UUID, currentProjectCount int, managedPassphrase bool, hubspotObjectID, tenantID *string) {
}

// TrackProjectDeleted implements Service.
func (n *NoopService) TrackProjectDeleted(userID uuid.UUID, email string, publicProjectID uuid.UUID, currentMonthUsage string, hubspotObjectID, tenantID *string) {
}

// TrackLegacyProjectTiersMigrated implements Service.
func (n *NoopService) TrackLegacyProjectTiersMigrated(userID uuid.UUID, email string, publicProjectID uuid.UUID, newPlacementProductMapping string, hubspotObjectID, tenantID *string) {
}

// TrackManagedEncryptionError implements Service.
func (n *NoopService) TrackManagedEncryptionError(userID uuid.UUID, email string, projectID uuid.UUID, reason string, hubspotObjectID, tenantID *string) {
}

// TrackRequestLimitIncrease implements Service.
func (n *NoopService) TrackRequestLimitIncrease(userID uuid.UUID, email string, info LimitRequestInfo, hubspotObjectID, tenantID *string) {
}

// TrackAccountVerified implements Service.
func (n *NoopService) TrackAccountVerified(userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
}

// TrackEvent implements Service.
func (n *NoopService) TrackEvent(eventName string, userID uuid.UUID, email string, customProps map[string]string, hubspotObjectID, tenantID *string) {
}

// TrackErrorEvent implements Service.
func (n *NoopService) TrackErrorEvent(userID uuid.UUID, email, source, requestID string, statusCode int, hubspotObjectID, tenantID *string) {
}

// TrackLinkEvent implements Service.
func (n *NoopService) TrackLinkEvent(eventName string, userID uuid.UUID, email, link string, hubspotObjectID, tenantID *string) {
}

// TrackCreditCardAdded implements Service.
func (n *NoopService) TrackCreditCardAdded(userID uuid.UUID, email string, hubspotObjectID *string) {}

// PageVisitEvent implements Service.
func (n *NoopService) PageVisitEvent(pageName string, userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
}

// TrackProjectLimitError implements Service.
func (n *NoopService) TrackProjectLimitError(userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
}

// TrackProjectMemberAddition implements Service.
func (n *NoopService) TrackProjectMemberAddition(userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
}

// TrackProjectMemberDeletion implements Service.
func (n *NoopService) TrackProjectMemberDeletion(userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
}

// TrackExpiredCreditNeedsRemoval implements Service.
func (n *NoopService) TrackExpiredCreditNeedsRemoval(userID uuid.UUID, customerID, packagePlan string, hubspotObjectID *string) {
}

// TrackExpiredCreditRemoved implements Service.
func (n *NoopService) TrackExpiredCreditRemoved(userID uuid.UUID, customerID, packagePlan string, hubspotObjectID *string) {
}

// TrackInviteLinkSignup implements Service.
func (n *NoopService) TrackInviteLinkSignup(inviter, invitee string) {}

// TrackInviteLinkClicked implements Service.
func (n *NoopService) TrackInviteLinkClicked(inviter, invitee string) {}

// TrackUserUpgraded implements Service.
func (n *NoopService) TrackUserUpgraded(userID uuid.UUID, email string, expiration *time.Time, hubspotObjectID *string) {
}

// TrackAdminAuditEvent implements Service.
func (n *NoopService) TrackAdminAuditEvent(userID uuid.UUID, customProps map[string]interface{}) {}

// ValidateAccountObjectCreatedRequestSignature implements Service.
func (n *NoopService) ValidateAccountObjectCreatedRequestSignature(request AccountObjectCreatedRequest, signatureHeader, timestampHeader string) error {
	return nil
}

// GetAccessToken implements Service.
func (n *NoopService) GetAccessToken(ctx context.Context) (token string, err error) {
	return "", Error.New("analytics service is not enabled")
}

// TestSetSatelliteExternalAddress implements Service.
func (n *NoopService) TestSetSatelliteExternalAddress(address string) {}

// TrackAccountFrozen implements FreezeTracker.
func (n *NoopService) TrackAccountFrozen(userID uuid.UUID, email string, hubspotObjectID *string) {}

// TrackAccountUnfrozen implements FreezeTracker.
func (n *NoopService) TrackAccountUnfrozen(userID uuid.UUID, email string, hubspotObjectID *string) {
}

// TrackAccountUnwarned implements FreezeTracker.
func (n *NoopService) TrackAccountUnwarned(userID uuid.UUID, email string, hubspotObjectID *string) {
}

// TrackAccountFreezeWarning implements FreezeTracker.
func (n *NoopService) TrackAccountFreezeWarning(userID uuid.UUID, email string, hubspotObjectID *string) {
}

// TrackLargeUnpaidInvoice implements FreezeTracker.
func (n *NoopService) TrackLargeUnpaidInvoice(invID string, userID uuid.UUID, email string, hubspotObjectID *string) {
}

// TrackViolationFrozenUnpaidInvoice implements FreezeTracker.
func (n *NoopService) TrackViolationFrozenUnpaidInvoice(invID string, userID uuid.UUID, email string, hubspotObjectID *string) {
}

// TrackLegalHoldUnpaidInvoice implements FreezeTracker.
func (n *NoopService) TrackLegalHoldUnpaidInvoice(invID string, userID uuid.UUID, email string, hubspotObjectID *string) {
}

// TrackStorjscanUnpaidInvoice implements FreezeTracker.
func (n *NoopService) TrackStorjscanUnpaidInvoice(invID string, userID uuid.UUID, email string, hubspotObjectID *string) {
}

// TrackGenericFreeze implements FreezeTracker.
func (n *NoopService) TrackGenericFreeze(userID uuid.UUID, email, freezeType string, adminInitiated bool, hubspotObjectID *string) {
}

// TrackGenericUnfreeze implements FreezeTracker.
func (n *NoopService) TrackGenericUnfreeze(userID uuid.UUID, email, freezeType string, adminInitiated bool, hubspotObjectID *string) {
}

var _ Service = (*NoopService)(nil)
