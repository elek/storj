// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <UpgradeAccountWrapper title="Add Credit Card">
        <template #content>
            <p class="card-info">
                By saving your card information, you allow Storj to charge your card for future payments in accordance with
                the terms.
            </p>
            <StripeCardElement
                v-if="paymentElementEnabled"
                ref="stripeCardInput"
                @pm-created="addCardToDB"
            />
            <StripeCardInput
                v-else
                ref="stripeCardInput"
                :on-stripe-response-callback="addCardToDB"
            />
            <VButton
                class="button"
                label="Save card"
                icon="lock"
                width="100%"
                height="48px"
                border-radius="10px"
                font-size="14px"
                :is-green="true"
                :on-press="onSaveCardClick"
                :is-disabled="loading"
            />
            <p class="security-info">Your information is secured with 128-bit SSL & AES-256 encryption.</p>
        </template>
    </UpgradeAccountWrapper>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { RouteConfig } from '@/types/router';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';

import UpgradeAccountWrapper from '@/components/modals/upgradeAccountFlow/UpgradeAccountWrapper.vue';
import VButton from '@/components/common/VButton.vue';
import StripeCardElement from '@/components/account/billing/paymentMethods/StripeCardElement.vue';
import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';

interface StripeForm {
    onSubmit(): Promise<void>;
}

const analyticsStore = useAnalyticsStore();
const usersStore = useUsersStore();
const billingStore = useBillingStore();
const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();
const route = useRoute();

const props = defineProps<{
    setSuccess: () => void;
}>();

const loading = ref<boolean>(false);
const stripeCardInput = ref<StripeForm | null>(null);

/**
 * Indicates whether stripe payment element is enabled.
 */
const paymentElementEnabled = computed(() => {
    return configStore.state.config.stripePaymentElementEnabled;
});

/**
 * Provides card information to Stripe.
 */
async function onSaveCardClick(): Promise<void> {
    if (loading.value || !stripeCardInput.value) return;

    loading.value = true;

    try {
        await stripeCardInput.value.onSubmit();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
        loading.value = false;
    }
}

/**
 * Adds card after Stripe confirmation.
 *
 * @param res - the response from stripe. Could be a token or a payment method id.
 * depending on the paymentElementEnabled flag.
 */
async function addCardToDB(res: string): Promise<void> {
    try {
        const action = paymentElementEnabled.value ? billingStore.addCardByPaymentMethodID : billingStore.addCreditCard;
        await action(res);
        notify.success('Card successfully added');
        // We fetch User one more time to update their Paid Tier status.
        await usersStore.getUser();

        if (route.name === RouteConfig.ProjectDashboard.name) {
            await projectsStore.getProjectLimits(projectsStore.state.selectedProject.id);
        }

        if (route.path.includes(RouteConfig.Billing.path) || route.path.includes(RouteConfig.Billing2.path)) {
            await billingStore.getCreditCards();
        }

        analyticsStore.eventTriggered(AnalyticsEvent.MODAL_ADD_CARD);

        loading.value = false;
        props.setSuccess();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
        loading.value = false;
    }
}
</script>

<style scoped lang="scss">
.card-info {
    font-family: 'font_regular', sans-serif;
    font-size: 14px;
    line-height: 20px;
    color: var(--c-blue-6);
    padding-bottom: 16px;
    margin-bottom: 16px;
    border-bottom: 1px solid var(--c-grey-2);
    text-align: left;
    max-width: 400px;
}

.button {
    margin: 16px 0;
}

.security-info {
    font-family: 'font_regular', sans-serif;
    font-size: 12px;
    line-height: 18px;
    text-align: center;
    color: var(--c-black);
}
</style>
