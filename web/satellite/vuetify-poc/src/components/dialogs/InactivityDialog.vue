// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="410px"
        transition="fade-transition"
    >
        <v-card rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <template #prepend>
                    <img class="d-block" src="@poc/assets/icon-session-timeout.svg" alt="Session expiring">
                </template>
                <v-card-title class="font-weight-bold">Session Expiring</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item class="pa-8">
                Your session is about to expire due to inactivity in:
                <br>
                <span class="font-weight-bold">{{ seconds }} second{{ seconds !== 1 ? 's' : '' }}</span>
                <br><br>
                Do you want to stay logged in?
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            :loading="isLogOutLoading"
                            @click="logOutClick"
                        >
                            Log out
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :loading="isContinueLoading"
                            :disabled="isLoading"
                            @click="continueClick"
                        >
                            Stay logged in
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { VDialog, VCard, VCardItem, VCardTitle, VBtn, VDivider, VCardActions, VRow, VCol } from 'vuetify/components';

import { INACTIVITY_MODAL_DURATION } from '@/composables/useSessionTimeout';

const props = defineProps<{
    modelValue: boolean,
    onContinue: () => Promise<void>;
    onLogout: () => Promise<void>;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean],
}>();

const seconds = ref<number>(0);
const isLogOutLoading = ref<boolean>(false);
const isContinueLoading = ref<boolean>(false);
const intervalId = ref<ReturnType<typeof setInterval> | null>(null);

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

/**
 * Indicates whether the dialog is processing an action.
 */
const isLoading = computed<boolean>(() => isLogOutLoading.value || isContinueLoading.value);

/**
 * Invokes the logout callback when the 'Log out' button has been clicked.
 */
async function logOutClick(): Promise<void> {
    if (isLoading.value) return;
    isLogOutLoading.value = true;
    await props.onLogout();
    isLogOutLoading.value = false;
}

/**
 * Invokes the continue callback when the 'Stay logged in' button has been clicked.
 */
async function continueClick(): Promise<void> {
    if (isLoading.value) return;
    isContinueLoading.value = true;
    await props.onContinue();
    isContinueLoading.value = false;
}

/**
 * Starts timer that decreases number of seconds until session expiration.
 */
watch(() => props.modelValue, shown => {
    if (!shown) {
        if (intervalId.value) clearInterval(intervalId.value);
        return;
    }
    seconds.value = INACTIVITY_MODAL_DURATION / 1000;
    intervalId.value = setInterval(() => {
        if (--seconds.value <= 0 && intervalId.value) clearInterval(intervalId.value);
    }, 1000);
}, { immediate: true });
</script>
