// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="9" md="7" lg="5">
                <v-card title="Check your email" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <p class="mb-4">We have sent you the password reset instructions to your email address.</p>
                        <v-btn color="primary" size="large" block router-link to="/login">
                            Go to login
                        </v-btn>
                    </v-card-text>
                </v-card>
                <p class="pt-6 text-center text-body-2">Didn’t get the password reset email?</p>
                <p class="text-center text-body-2">Make sure you selected the correct satellite.</p>
                <p class="text-center text-body-2">Go back to <router-link class="link" to="/password-reset">password reset</router-link>.</p>
                <p class="pt-6 text-center text-body-2">Still having problems? <router-link class="link" to="/login">Contact Support</router-link></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VCol, VContainer, VRow } from 'vuetify/components';
import { ref } from 'vue';

const valid = ref(false);
const checked = ref(false);
const email = ref('');
const select = ref({ satellite: 'US1' });
const items = ref([
    { satellite: 'US1' },
    { satellite: 'EU1' },
    { satellite: 'AP1' },
]);
</script>