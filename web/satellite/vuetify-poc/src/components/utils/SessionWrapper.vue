// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <slot v-bind="sessionTimeout" />

    <v-snackbar
        :model-value="sessionTimeout.debugTimerShown.value"
        :timeout="-1"
        color="warning"
        rounded="pill"
        min-width="0"
        location="top"
    >
        <v-icon icon="mdi-clock" />
        Remaining session time:
        <span class="font-weight-bold">{{ sessionTimeout.debugTimerText.value }}</span>
    </v-snackbar>

    <set-session-timeout-dialog v-model="isSetTimeoutModalShown" />
    <inactivity-dialog
        v-model="sessionTimeout.inactivityModalShown.value"
        :on-continue="() => sessionTimeout.refreshSession(true)"
        :on-logout="sessionTimeout.handleInactive"
    />
    <session-expired-dialog v-model="sessionTimeout.sessionExpiredModalShown.value" />
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VSnackbar, VIcon } from 'vuetify/lib/components/index.mjs';

import { useSessionTimeout } from '@/composables/useSessionTimeout';

import InactivityDialog from '@poc/components/dialogs/InactivityDialog.vue';
import SessionExpiredDialog from '@poc/components/dialogs/SessionExpiredDialog.vue';
import SetSessionTimeoutDialog from '@poc/components/dialogs/SetSessionTimeoutDialog.vue';

const isSetTimeoutModalShown = ref<boolean>(false);

const sessionTimeout = useSessionTimeout({
    showEditSessionTimeoutModal: () => isSetTimeoutModalShown.value = true,
});
</script>
