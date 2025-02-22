// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row v-if="isLoading" justify="center">
        <v-col cols="auto">
            <v-progress-circular indeterminate />
        </v-col>
    </v-row>
    <template v-else>
        <v-row :align="smAndDown ? 'center' : 'start'" :justify="smAndDown ? 'start' : 'space-between'" :class="{'flex-column': smAndDown}">
            <v-col v-for="(plan, index) in plans" :key="index" :cols="smAndDown ? 10 : 6">
                <PricingPlanContainer
                    :plan="plan"
                    @select="(p) => emit('select', p)"
                />
            </v-col>
        </v-row>
    </template>
</template>

<script setup lang="ts">
import { onBeforeMount, ref } from 'vue';
import { useRouter } from 'vue-router';
import { VCol, VProgressCircular, VRow } from 'vuetify/components';
import { useDisplay } from 'vuetify';

import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { User } from '@/types/users';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';

import PricingPlanContainer from '@poc/components/billing/pricingPlans/PricingPlanContainer.vue';

const usersStore = useUsersStore();
const router = useRouter();
const notify = useNotify();
const { smAndDown } = useDisplay();

const emit = defineEmits<{
  select: [PricingPlanInfo];
}>();

const isLoading = ref<boolean>(true);

const plans = ref<PricingPlanInfo[]>([
    new PricingPlanInfo(
        PricingPlanType.PRO,
        'Pro Account',
        '25 GB Free',
        'Only pay for what you need. $4/TB stored per month* $7/TB for egress bandwidth.',
        '*Additional per-segment fee of $0.0000088 applies.',
        null,
        null,
        'Add a credit card to activate your Pro Account.<br><br>Get 25GB free storage and egress. Only pay for what you use beyond that.',
        'No charge today',
        '25GB Free',
    ),
]);

/*
 * Loads pricing plan config. Assumes that user is already eligible for a plan prior to component being mounted.
 */
onBeforeMount(async () => {
    const user: User = usersStore.state.user;

    let config;
    try {
        config = (await import('@poc/components/billing/pricingPlans/pricingPlanConfig.json')).default;
    } catch {
        notify.error('No pricing plan configuration file.', null);
        return;
    }

    const plan = config[user.partner] as PricingPlanInfo;
    if (!plan) {
        notify.error(`No pricing plan configuration for partner '${user.partner}'.`, null);
        return;
    }
    plan.type = PricingPlanType.PARTNER;
    plans.value.unshift(plan);

    isLoading.value = false;
});
</script>