// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package accountfreeze_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v75"
	"go.uber.org/zap"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/storjscan/blockchaintest"
	stripe1 "storj.io/storj/satellite/payments/stripe"
)

func TestAutoFreezeChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		stripeClient := sat.API.Payments.StripeClient
		invoicesDB := sat.Core.Payments.Accounts.Invoices()
		customerDB := sat.Core.DB.StripeCoinPayments().Customers()
		usersDB := sat.DB.Console().Users()
		service := console.NewAccountFreezeService(sat.DB.Console(), newFreezeTrackerMock(t), sat.Config.Console.AccountFreeze)
		chore := sat.Core.Payments.AccountFreeze

		chore.Loop.Pause()
		chore.TestSetFreezeService(service)

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "user@mail.test",
		}, 1)
		require.NoError(t, err)

		cus1, err := customerDB.GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		amount := int64(100)
		curr := string(stripe.CurrencyUSD)

		t.Run("No billing event for legal frozen user", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))

			violatingUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Violating User",
				Email:    "legalhold@mail.test",
			}, 1)
			require.NoError(t, err)

			cus2, err := customerDB.GetCustomerID(ctx, violatingUser.ID)
			require.NoError(t, err)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cus2,
			})
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus2,
				Invoice:  &inv.ID,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPayFailure
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.Error(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 1, len(failed))

			require.NoError(t, service.LegalFreezeUser(ctx, violatingUser.ID))

			chore.Loop.TriggerWait()

			// user should not be billing warned or frozen.
			freezes, err := service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingWarning)
			require.Nil(t, freezes.BillingFreeze)
			require.NotNil(t, freezes.LegalFreeze)

			// forward date to after the grace period
			chore.TestSetNow(func() time.Time {
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingWarnGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user should still not be billing warned or frozen.
			freezes, err = service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.NotNil(t, freezes.LegalFreeze)

			paymentMethod = stripe1.MockInvoicesPaySuccess
			_, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusPaid, inv.Status)

			chore.Loop.TriggerWait()

			// paying for the invoice does not remove the legal freeze
			freezes, err = service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.NotNil(t, freezes.LegalFreeze)
		})

		t.Run("No billing event for violation frozen user", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))

			violatingUser, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Violating User",
				Email:    "violating@mail.test",
			}, 1)
			require.NoError(t, err)

			cus2, err := customerDB.GetCustomerID(ctx, violatingUser.ID)
			require.NoError(t, err)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cus2,
			})
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus2,
				Invoice:  &inv.ID,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPayFailure
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.Error(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 1, len(failed))

			require.NoError(t, service.ViolationFreezeUser(ctx, violatingUser.ID))

			chore.Loop.TriggerWait()

			// user should not be billing warned or frozen.
			freezes, err := service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingWarning)
			require.Nil(t, freezes.BillingFreeze)
			require.NotNil(t, freezes.ViolationFreeze)

			// forward date to after the grace period
			chore.TestSetNow(func() time.Time {
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingWarnGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user should still not be billing warned or frozen.
			freezes, err = service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.NotNil(t, freezes.ViolationFreeze)

			paymentMethod = stripe1.MockInvoicesPaySuccess
			_, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusPaid, inv.Status)

			chore.Loop.TriggerWait()

			// paying for the invoice does not remove the violation freeze
			freezes, err = service.GetAll(ctx, violatingUser.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.NotNil(t, freezes.ViolationFreeze)
		})

		t.Run("No billing freeze event for paid invoice", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			_, err := stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus1,
			})
			require.NoError(t, err)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cus1,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPaySuccess
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusPaid, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 0, len(failed))

			chore.Loop.TriggerWait()

			// user should not be warned or frozen.
			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.Nil(t, freezes.ViolationFreeze)

			// forward date to after the grace period
			chore.TestSetNow(func() time.Time {
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingWarnGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user should still not be warned or frozen.
			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.BillingWarning)
			require.Nil(t, freezes.ViolationFreeze)
		})

		t.Run("BillingFreeze event for failed invoice (failed later payment attempt)", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			// reset chore clock
			chore.TestSetNow(time.Now)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cus1,
			})
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus1,
				Invoice:  &inv.ID,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPayFailure
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.Error(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 1, len(failed))
			require.Equal(t, inv.ID, failed[0].ID)

			chore.Loop.TriggerWait()

			// user should be warned the first time
			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.BillingWarning)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.ViolationFreeze)

			chore.TestSetNow(func() time.Time {
				// current date is now after billing warn grace period
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingWarnGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user should be frozen this time around
			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.BillingFreeze)

			chore.TestSetNow(func() time.Time {
				// current date is now after billing freeze grace period
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingFreezeGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// user should be marked for deletion after the grace period
			// after being frozen
			userPD, err := usersDB.Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, console.PendingDeletion, userPD.Status)

			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.BillingFreeze)
			// the billing freeze event should have escalation disabled.
			require.Nil(t, freezes.BillingFreeze.DaysTillEscalation)

			// Pay invoice so user qualifies to be removed from billing freeze.
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: stripe.String(stripe1.MockInvoicesPaySuccess),
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusPaid, inv.Status)

			// set user status to deleted
			status := console.Deleted
			err = usersDB.Update(ctx, user.ID, console.UpdateUserRequest{
				Status: &status,
			})
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// deleted user should be skipped, hence would not exist the
			// billing freeze status.
			isFrozen, err := service.IsUserBillingFrozen(ctx, user.ID)
			require.NoError(t, err)
			require.True(t, isFrozen)

			// unfreeze user so they're not frozen in the next test.
			err = service.BillingUnfreezeUser(ctx, user.ID)
			require.NoError(t, err)

			// set user status back to active
			status = console.Active
			err = usersDB.Update(ctx, user.ID, console.UpdateUserRequest{
				Status: &status,
			})
			require.NoError(t, err)
		})

		t.Run("No freeze event for failed invoice (successful later payment attempt)", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			// reset chore clock
			chore.TestSetNow(time.Now)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:               stripe.Params{Context: ctx},
				Customer:             &cus1,
				DefaultPaymentMethod: stripe.String(stripe1.MockInvoicesPaySuccess),
			})
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus1,
				Invoice:  &inv.ID,
			})
			require.NoError(t, err)

			inv, err = stripeClient.Invoices().FinalizeInvoice(inv.ID, nil)
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 1, len(failed))
			require.Equal(t, inv.ID, failed[0].ID)

			chore.Loop.TriggerWait()

			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.BillingWarning)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.ViolationFreeze)

			chore.TestSetNow(func() time.Time {
				// current date is now after billing warn grace period
				return time.Now().Add(sat.Config.Console.AccountFreeze.BillingWarnGracePeriod).Add(24 * time.Hour)
			})
			chore.Loop.TriggerWait()

			// Payment should have succeeded in the chore.
			failed, err = invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 0, len(failed))

			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.BillingWarning)
			require.Nil(t, freezes.BillingFreeze)
			require.Nil(t, freezes.ViolationFreeze)
		})

		t.Run("User unfrozen/unwarned for no failed invoices", func(t *testing.T) {
			// AnalyticsMock tests that events are sent once.
			service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
			// reset chore clock
			chore.TestSetNow(time.Now)

			user2, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User",
				Email:    "user2@mail.test",
			}, 1)
			require.NoError(t, err)

			cus2, err := customerDB.GetCustomerID(ctx, user2.ID)
			require.NoError(t, err)

			inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
				Params:   stripe.Params{Context: ctx},
				Customer: &cus2,
			})
			require.NoError(t, err)

			_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
				Params:   stripe.Params{Context: ctx},
				Amount:   &amount,
				Currency: &curr,
				Customer: &cus2,
				Invoice:  &inv.ID,
			})
			require.NoError(t, err)

			paymentMethod := stripe1.MockInvoicesPayFailure
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: &paymentMethod,
			})
			require.Error(t, err)
			require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

			failed, err := invoicesDB.ListFailed(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, 1, len(failed))

			err = service.BillingFreezeUser(ctx, user.ID)
			require.NoError(t, err)
			err = service.BillingFreezeUser(ctx, user2.ID)
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// user(1) should be unfrozen because they have no failed invoices
			freezes, err := service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.BillingFreeze)

			// user2 should still be frozen because they have failed invoices
			freezes, err = service.GetAll(ctx, user2.ID)
			require.NoError(t, err)
			require.NotNil(t, freezes.BillingFreeze)

			// warn user though they have no failed invoices
			err = service.BillingWarnUser(ctx, user.ID)
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// warned status should be reset
			freezes, err = service.GetAll(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, freezes.BillingWarning)

			// Pay invoice so it doesn't show up in the next test.
			inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
				Params:        stripe.Params{Context: ctx},
				PaymentMethod: stripe.String(stripe1.MockInvoicesPaySuccess),
			})
			require.NoError(t, err)
			require.Equal(t, stripe.InvoiceStatusPaid, inv.Status)

			// unfreeze user so they're not frozen in the next test.
			err = service.BillingUnfreezeUser(ctx, user2.ID)
			require.NoError(t, err)
		})
	})
}

func TestAutoFreezeChore_StorjscanExclusion(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.AccountFreeze.Enabled = true
				config.AccountFreeze.ExcludeStorjscan = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		stripeClient := sat.API.Payments.StripeClient
		invoicesDB := sat.Core.Payments.Accounts.Invoices()
		customerDB := sat.Core.DB.StripeCoinPayments().Customers()
		service := console.NewAccountFreezeService(sat.DB.Console(), newFreezeTrackerMock(t), sat.Config.Console.AccountFreeze)
		chore := sat.Core.Payments.AccountFreeze

		chore.Loop.Pause()
		chore.TestSetFreezeService(service)

		amount := int64(100)
		curr := string(stripe.CurrencyUSD)

		// AnalyticsMock tests that events are sent once.
		service.TestChangeFreezeTracker(newFreezeTrackerMock(t))
		// reset chore clock
		chore.TestSetNow(time.Now)

		storjscanUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "storjscanuser@mail.test",
		}, 1)
		require.NoError(t, err)

		// create a wallet and transaction for the new user in storjscan
		address, err := blockchain.BytesToAddress(testrand.Bytes(20))
		require.NoError(t, err)
		require.NoError(t, sat.DB.Wallets().Add(ctx, storjscanUser.ID, address))
		cachedPayments := []storjscan.CachedPayment{
			{
				From:        blockchaintest.NewAddress(),
				To:          address,
				TokenValue:  currency.AmountFromBaseUnits(1000, currency.StorjToken),
				USDValue:    currency.AmountFromBaseUnits(testrand.Int63n(1000), currency.USDollarsMicro),
				BlockHash:   blockchaintest.NewHash(),
				Transaction: blockchaintest.NewHash(),
				Status:      payments.PaymentStatusConfirmed,
				Timestamp:   time.Now(),
			},
		}
		require.NoError(t, sat.DB.StorjscanPayments().InsertBatch(ctx, cachedPayments))

		storjscanCus, err := customerDB.GetCustomerID(ctx, storjscanUser.ID)
		require.NoError(t, err)

		inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &storjscanCus,
		})
		require.NoError(t, err)

		_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Amount:   &amount,
			Currency: &curr,
			Customer: &storjscanCus,
			Invoice:  &inv.ID,
		})
		require.NoError(t, err)

		paymentMethod := stripe1.MockInvoicesPayFailure
		inv, err = stripeClient.Invoices().Pay(inv.ID, &stripe.InvoicePayParams{
			Params:        stripe.Params{Context: ctx},
			PaymentMethod: &paymentMethod,
		})
		require.Error(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

		failed, err := invoicesDB.ListFailed(ctx, nil)
		require.NoError(t, err)
		require.Equal(t, 1, len(failed))
		invFound := false
		for _, failedInv := range failed {
			if failedInv.ID == inv.ID {
				invFound = true
				break
			}
		}
		require.True(t, invFound)

		chore.Loop.TriggerWait()

		// user should not be warned or frozen due to storjscan payments
		freezes, err := service.GetAll(ctx, storjscanUser.ID)
		require.NoError(t, err)
		require.Nil(t, freezes.BillingWarning)
		require.Nil(t, freezes.BillingFreeze)
		require.Nil(t, freezes.ViolationFreeze)
	})
}

type freezeTrackerMock struct {
	t            *testing.T
	freezeCounts map[string]int
	warnCounts   map[string]int
}

func newFreezeTrackerMock(t *testing.T) *freezeTrackerMock {
	return &freezeTrackerMock{
		t:            t,
		freezeCounts: map[string]int{},
		warnCounts:   map[string]int{},
	}
}

// The following functions are implemented from analytics.FreezeTracker.
// They mock/test to make sure freeze events are sent just once.

func (mock *freezeTrackerMock) TrackAccountFrozen(_ uuid.UUID, email string) {
	mock.freezeCounts[email]++
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 1, mock.freezeCounts[email])
}

func (mock *freezeTrackerMock) TrackAccountUnfrozen(_ uuid.UUID, email string) {
	mock.freezeCounts[email]--
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 0, mock.freezeCounts[email])
}

func (mock *freezeTrackerMock) TrackAccountUnwarned(_ uuid.UUID, email string) {
	mock.warnCounts[email]--
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 0, mock.warnCounts[email])
}

func (mock *freezeTrackerMock) TrackAccountFreezeWarning(_ uuid.UUID, email string) {
	mock.warnCounts[email]++
	// make sure this tracker has not been called already for this email.
	require.Equal(mock.t, 1, mock.warnCounts[email])
}

func (mock *freezeTrackerMock) TrackLargeUnpaidInvoice(_ string, _ uuid.UUID, _ string) {}

func (mock *freezeTrackerMock) TrackStorjscanUnpaidInvoice(_ string, _ uuid.UUID, _ string) {}

func (mock *freezeTrackerMock) TrackViolationFrozenUnpaidInvoice(_ string, _ uuid.UUID, _ string) {}
