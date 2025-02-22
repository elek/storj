// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="all-dashboard-banners">
        <UpgradeNotification
            v-if="isPaidTierBannerShown"
            class="all-dashboard-banners__upgrade"
            :open-add-p-m-modal="togglePMModal"
        />

        <v-banner
            v-if="isAccountFrozen && parentRef && billingEnabled"
            class="all-dashboard-banners__freeze"
            title="Your account was frozen due to billing issues."
            message="Please update your payment information."
            severity="critical"
            link-text="To Billing Page"
            :dashboard-ref="parentRef"
            @link-click="redirectToBillingPage"
        />

        <v-banner
            v-if="isAccountWarned && parentRef && billingEnabled"
            class="all-dashboard-banners__warning"
            title="Your account will be frozen soon due to billing issues."
            message="Please update your payment information."
            link-text="To Billing Page"
            severity="warning"
            :dashboard-ref="parentRef"
            :on-link-click="redirectToBillingPage"
        />

        <v-banner
            v-if="isLowBalance && parentRef && billingEnabled"
            class="all-dashboard-banners__low-balance"
            message="Your STORJ Token balance is low. Deposit more STORJ tokens or make sure you have a credit card on file to avoid interruptions in service."
            link-text="Go to billing"
            severity="warning"
            :dashboard-ref="parentRef"
            :on-link-click="redirectToBillingOverview"
        />
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { useUsersStore } from '@/store/modules/usersStore';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { RouteConfig } from '@/types/router';
import { useConfigStore } from '@/store/modules/configStore';
import { useLowTokenBalance } from '@/composables/useLowTokenBalance';

import VBanner from '@/components/common/VBanner.vue';
import UpgradeNotification from '@/components/notifications/UpgradeNotification.vue';

const router = useRouter();

const usersStore = useUsersStore();
const appStore = useAppStore();
const configStore = useConfigStore();

const isLowBalance = useLowTokenBalance();

const props = defineProps<{
  parentRef: HTMLElement;
}>();

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.state.config.billingFeaturesEnabled);

/**
 * Indicates if account was frozen due to billing issues.
 */
const isAccountFrozen = computed((): boolean => {
    return usersStore.state.user.freezeStatus.frozen;
});

/**
 * Indicates if account was warned due to billing issues.
 */
const isAccountWarned = computed((): boolean => {
    return usersStore.state.user.freezeStatus.warned;
});

/* whether the paid tier banner should be shown */
const isPaidTierBannerShown = computed((): boolean => {
    return !usersStore.state.user.paidTier
        && joinedWhileAgo.value
        && billingEnabled.value;
});

/* whether the user joined more than 7 days ago */
const joinedWhileAgo = computed((): boolean => {
    const createdAt = usersStore.state.user.createdAt as Date | null;
    if (!createdAt) return true; // true so we can show the banner regardless
    const millisPerDay = 24 * 60 * 60 * 1000;
    return ((Date.now() - createdAt.getTime()) / millisPerDay) > 7;
});

/**
 * Opens add payment method modal.
 */
function togglePMModal(): void {
    if (usersStore.state.user.paidTier || !billingEnabled.value) return;
    appStore.updateActiveModal(MODALS.upgradeAccount);
}

/**
 * Redirects to Billing Page.
 */
async function redirectToBillingPage(): Promise<void> {
    await router.push(RouteConfig.AccountSettings.with(RouteConfig.Billing2.with(RouteConfig.BillingPaymentMethods2)).path);
}

/**
 * Redirects to Billing Page Overview tab.
 */
async function redirectToBillingOverview(): Promise<void> {
    await router.push(RouteConfig.AccountSettings.with(RouteConfig.Billing2.with(RouteConfig.BillingOverview2)).path);
}
</script>

<style scoped lang="scss">
.all-dashboard-banners {
    margin-bottom: 20px;

    &__upgrade,
    &__freeze,
    &__warning,
    &__low-balance {
        margin: 20px 0 0;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
    }
}
</style>
