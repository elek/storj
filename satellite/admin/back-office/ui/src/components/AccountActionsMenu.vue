// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-menu activator="parent">
        <v-list class="pa-2">
            <v-list-item density="comfortable" link rounded="lg" base-color="info" router-link to="/account-details">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    View Account
                </v-list-item-title>
            </v-list-item>

            <v-divider class="my-2" />

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Edit Account
                    <AccountInformationDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Status
                    <AccountStatusDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Value
                    <AccountUserAgentsDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Set Placement
                    <AccountGeofenceDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Change Limits
                    <AccountLimitsDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    New Project
                    <AccountNewProjectDialog />
                </v-list-item-title>
            </v-list-item>

            <v-divider class="my-2" />

            <v-list-item density="comfortable" link rounded="lg">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Reset MFA
                    <AccountResetMFADialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg" base-color="warning">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Suspend
                    <AccountSuspendDialog />
                </v-list-item-title>
            </v-list-item>

            <v-list-item density="comfortable" link rounded="lg" base-color="error">
                <v-list-item-title class="text-body-2 font-weight-medium">
                    Delete
                    <AccountDeleteDialog />
                </v-list-item-title>
            </v-list-item>
        </v-list>
    </v-menu>
</template>

<script setup lang="ts">
import { VMenu, VList, VListItem, VListItemTitle, VDivider } from 'vuetify/components';

import AccountInformationDialog from '@/components/AccountInformationDialog.vue';
import AccountStatusDialog from '@/components/AccountStatusDialog.vue';
import AccountResetMFADialog from '@/components/AccountResetMFADialog.vue';
import AccountSuspendDialog from '@/components/AccountSuspendDialog.vue';
import AccountDeleteDialog from '@/components/AccountDeleteDialog.vue';
import AccountNewProjectDialog from '@/components/AccountNewProjectDialog.vue';
import AccountGeofenceDialog from '@/components/AccountGeofenceDialog.vue';
import AccountUserAgentsDialog from '@/components/AccountUserAgentsDialog.vue';
import AccountLimitsDialog from '@/components/AccountLimitsDialog.vue';
</script>
