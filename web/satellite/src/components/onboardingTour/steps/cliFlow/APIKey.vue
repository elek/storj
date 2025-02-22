// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="API Key Generated"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="key">
            <p class="key__msg">Now copy and save the Satellite Address and API Key as it will only appear once.</p>
            <h3 class="key__label">Satellite Address</h3>
            <ValueWithCopy label="Satellite Address" role-description="satellite-address" :value="satelliteAddress" />
            <h3 class="key__label">API Key</h3>
            <ValueWithCopy label="API Key" role-description="api-key" :value="storedAPIKey" />
        </template>
    </CLIFlowContainer>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import CLIFlowContainer from '@/components/onboardingTour/steps/common/CLIFlowContainer.vue';
import ValueWithCopy from '@/components/onboardingTour/steps/common/ValueWithCopy.vue';

import Icon from '@/../static/images/onboardingTour/apiKeyStep.svg';

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const appStore = useAppStore();
const router = useRouter();

/**
 * Returns the web address of this satellite from the store.
 */
const satelliteAddress = computed((): string => {
    return configStore.state.config.satelliteNodeURL;
});

/**
 * Returns API key from store.
 */
const storedAPIKey = computed((): string => {
    return appStore.state.onbApiKey;
});

/**
 * Returns back route from store.
 */
const backRoute = computed((): string => {
    return appStore.state.onbAPIKeyStepBackRoute;
});

/**
 * Holds on back button click logic.
 * Navigates to previous screen.
 */
async function onBackClick(): Promise<void> {
    if (backRoute.value) {
        analyticsStore.pageVisit(backRoute.value);
        await router.push(backRoute.value).catch(() => {return; });

        return;
    }

    analyticsStore.pageVisit(RouteConfig.OnboardingTour.path);
    await router.push(RouteConfig.OnboardingTour.path).catch(() => {return; });
}

/**
 * Holds on next button click logic.
 */
async function onNextClick(): Promise<void> {
    analyticsStore.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLIInstall)).path);
    await router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLIInstall)).path);
}

/**
 * Lifecycle hook after initial render.
 * Checks if api key was generated during previous step.
 */
onMounted((): void => {
    if (!storedAPIKey.value) {
        analyticsStore.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.AGName)).path);
        router.push({ name: RouteConfig.AGName.name });
    }
});
</script>

<style scoped lang="scss">
    .key {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 16px;
            line-height: 24px;
            color: #1b2533;
        }

        &__label {
            font-family: 'font_bold', sans-serif;
            font-size: 14px;
            line-height: 20px;
            color: var(--c-grey-6);
            margin: 20px 0 13px;
            width: 100%;
        }
    }
</style>
