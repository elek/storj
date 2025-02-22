// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="320px"
        max-width="460px"
        transition="fade-transition"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <template #prepend>
                    <img class="d-block" src="@poc/assets/icon-mfa.svg" alt="MFA">
                </template>
                <v-card-title class="font-weight-bold">Setup Two-Factor</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>
            <v-divider />
            <v-window v-model="step" :class="{ 'overflow-y-auto': step === 0 }">
                <!-- QR code step -->
                <v-window-item :value="0">
                    <v-card-item class="pa-7">
                        <p>Scan this QR code in your two-factor application.</p>
                    </v-card-item>
                    <v-card-item align="center" justify="center" class="rounded-lg border mx-7">
                        <v-col cols="auto">
                            <canvas ref="canvas" />
                        </v-col>
                    </v-card-item>
                    <v-card-item class="pa-7">
                        <p>Unable to scan? Enter the following code instead.</p>
                    </v-card-item>
                    <v-card-item class="rounded-lg border mx-7 mb-7 py-2">
                        <v-col>
                            <p class="font-weight-medium text-body-2 text-center"> {{ userMFASecret }}</p>
                        </v-col>
                    </v-card-item>
                </v-window-item>

                <!-- Enter code step -->
                <v-window-item :value="1">
                    <v-card-item class="pa-7">
                        <p>Enter the authentication code generated in your two-factor application to confirm your setup.</p>
                        <v-form v-model="formValid" class="pt-7" @submit.prevent>
                            <v-text-field
                                v-model="confirmPasscode"
                                variant="outlined"
                                hint="Example: 123456"
                                :rules="rules"
                                :error-messages="isError ? 'Invalid code. Please re-enter.' : ''"
                                label="2FA Code"
                                :hide-details="false"
                                maxlength="6"
                                required
                                autofocus
                            />
                        </v-form>
                    </v-card-item>
                </v-window-item>

                <!-- Save codes step -->
                <v-window-item :value="2">
                    <v-card-item class="px-7 py-4">
                        <p>Please save these codes somewhere to be able to recover access to your account.</p>
                    </v-card-item>
                    <v-divider />
                    <v-card-item class="px-7 py-4">
                        <p
                            v-for="(code, index) in userMFARecoveryCodes"
                            :key="index"
                        >
                            {{ code }}
                        </p>
                    </v-card-item>
                </v-window-item>
            </v-window>
            <v-divider />
            <v-card-actions class="pa-7">
                <v-row>
                    <v-col v-if="step !== 2">
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            :loading="isLoading"
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            v-if="step === 0"
                            color="primary"
                            variant="flat"
                            block
                            :loading="isLoading"
                            @click="step++"
                        >
                            Continue
                        </v-btn>
                        <v-btn
                            v-else-if="step === 1"
                            color="primary"
                            variant="flat"
                            block
                            :loading="isLoading"
                            :disabled="!formValid"
                            @click="enable"
                        >
                            Confirm
                        </v-btn>

                        <v-btn
                            v-else
                            color="primary"
                            variant="flat"
                            block
                            @click="model = false"
                        >
                            Done
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VRow,
    VTextField,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import QRCode from 'qrcode';

import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';

const rules = [
    (value: string) => (!!value || 'Can\'t be empty'),
    (value: string) => (!value.includes(' ') || 'Can\'t contain spaces'),
    (value: string) => (!!parseInt(value) || 'Can only be numbers'),
    (value: string) => (value.length === 6 || 'Can only be 6 numbers long'),
];

const analyticsStore = useAnalyticsStore();
const { config } = useConfigStore().state;
const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const canvas = ref<HTMLCanvasElement>();
const innerContent = ref<Component | null>(null);

const props = defineProps<{
    modelValue: boolean,
}>();

const emit = defineEmits<{
    (event: 'update:modelValue', value: boolean): void,
}>();

const step = ref<number>(0);
const confirmPasscode = ref<string>('');
const isError = ref<boolean>(false);
const formValid = ref<boolean>(false);

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

/**
 * Returns pre-generated MFA secret from store.
 */
const userMFASecret = computed((): string => {
    return usersStore.state.userMFASecret;
});

/**
 * Returns user MFA recovery codes from store.
 */
const userMFARecoveryCodes = computed((): string[] => {
    return usersStore.state.userMFARecoveryCodes;
});

/**
 * Returns satellite name from store.
 */
const satellite = computed((): string => {
    return config.satelliteName;
});

/**
 * Returns the 2FA QR link.
 */
const qrLink = computed((): string => {
    return `otpauth://totp/${encodeURIComponent(usersStore.state.user.email)}?secret=${userMFASecret.value}&issuer=${encodeURIComponent(`STORJ ${satellite.value}`)}&algorithm=SHA1&digits=6&period=30`;
});

/**
 * Enables user MFA and sets view to Recovery Codes state.
 */
function enable(): void {
    if (!formValid.value) return;

    withLoading(async () => {
        try {
            await usersStore.enableUserMFA(confirmPasscode.value);
            await usersStore.getUser();
            await showCodes();

            analyticsStore.eventTriggered(AnalyticsEvent.MFA_ENABLED);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ENABLE_MFA_MODAL);
            isError.value = true;
        }
    });
}

/**
 * Toggles view to MFA Recovery Codes state.
 */
async function showCodes() {
    try {
        await usersStore.generateUserMFARecoveryCodes();
        step.value = 2;
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ENABLE_MFA_MODAL);
    }
}

watch(canvas, async val => {
    if (!val) return;
    try {
        await QRCode.toCanvas(canvas.value, qrLink.value);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ENABLE_MFA_MODAL);
    }
});

watch(confirmPasscode, () => {
    isError.value = false;
});

watch(innerContent, newContent => {
    if (newContent) return;
    // dialog has been closed
    if (step.value === 2) {
        // recovery codes/success step
        notify.success('2FA successfully enabled');
    }
    step.value = 0;
    confirmPasscode.value = '';
    isError.value = false;
});
</script>