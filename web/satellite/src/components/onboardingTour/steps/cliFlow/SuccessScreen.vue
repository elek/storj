// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="success-screen">
        <Icon />
        <h1 class="success-screen__title" aria-roledescription="title">
            Wonderful, you have completed the Uplink CLI Quickstart Guide
        </h1>
        <p class="success-screen__msg">
            If you want to learn more, visit the
            <a class="link" href="https://docs.storj.io/" target="_blank" rel="noopener noreferrer">documentation</a>.
            You can also find a list of all the
            <a class="link" href="https://www.storj.io/integrations" target="_blank" rel="noopener noreferrer">integrations</a> on the website.
        </p>
        <div class="success-screen__buttons">
            <VButton
                label="Back"
                height="48px"
                :is-white="true"
                :on-press="onBackClick"
            />
            <VButton
                label="Finish"
                height="48px"
                :on-press="onFinishClick"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VButton from '@/components/common/VButton.vue';

import Icon from '@/../static/images/onboardingTour/successStep.svg';

const analyticsStore = useAnalyticsStore();
const usersStore = useUsersStore();
const notify = useNotify();
const router = useRouter();

/**
 * Holds on back button click logic.
 */
async function onBackClick(): Promise<void> {
    analyticsStore.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.ShareObject)).path);
    await router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.ShareObject)).path);
}

/**
 * Holds on finish button click logic.
 */
async function onFinishClick(): Promise<void> {
    endOnboarding();
    analyticsStore.pageVisit(RouteConfig.ProjectDashboard.path);
    await router.push(RouteConfig.ProjectDashboard.path);
}

async function endOnboarding(): Promise<void> {
    try {
        await usersStore.updateSettings({ onboardingEnd: true });
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ONBOARDING_OVERVIEW_STEP);
    }
}
</script>

<style scoped lang="scss">
    .success-screen {
        font-family: 'font_regular', sans-serif;
        background: #fcfcfc;
        box-shadow: 0 0 32px rgb(0 0 0 / 4%);
        border-radius: 20px;
        padding: 48px;
        max-width: 484px;
        display: flex;
        align-items: center;
        flex-direction: column;

        @media screen and (width <= 600px) {
            padding: 24px;
        }

        &__title {
            margin: 20px 0;
            font-family: 'font_Bold', sans-serif;
            font-size: 28px;
            line-height: 36px;
            text-align: center;
            letter-spacing: -0.02em;
            color: #14142b;
        }

        &__msg {
            font-size: 16px;
            line-height: 24px;
            color: #1b2533;
        }

        &__buttons {
            display: flex;
            align-items: center;
            width: 100%;
            margin-top: 24px;
            column-gap: 24px;

            @media screen and (width <= 450px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 24px;
            }
        }
    }

    .link {
        color: #1b2533;
        text-decoration: underline !important;
        text-underline-position: under;

        &:visited {
            color: #1b2533;
        }
    }
</style>
