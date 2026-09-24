// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Notifications" />
        <PageSubtitleComponent subtitle="Choose where you get notified when the status of your confirmed nodes changes." />

        <v-alert v-if="isForbidden" class="my-4" type="info" variant="tonal">
            Verify your email address to manage the notifications of your storage nodes.
        </v-alert>

        <v-row v-else class="mt-2">
            <v-col cols="12" sm="6" lg="4">
                <v-card title="Email" class="pa-2">
                    <v-card-text>
                        <v-chip color="primary" variant="tonal" size="small" class="font-weight-bold">
                            Always on
                        </v-chip>
                        <p class="mt-4">
                            Status changes are sent to the email address your nodes are registered with.
                        </p>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <v-card title="Telegram" class="pa-2" :loading="isLoading">
                    <v-card-text>
                        <template v-if="!isLoading && !telegram.enabled">
                            <v-chip variant="tonal" size="small" class="font-weight-bold">
                                Not available
                            </v-chip>
                            <p class="mt-4">Telegram notifications are not available on this satellite.</p>
                        </template>

                        <template v-else-if="!isLoading && telegram.connected">
                            <v-chip color="success" variant="tonal" size="small" class="font-weight-bold">
                                Connected
                            </v-chip>
                            <p class="mt-4">
                                Status changes of your confirmed nodes are sent to your chat with
                                <a :href="botURL" target="_blank" rel="noopener noreferrer">@{{ telegram.botName }}</a>.
                            </p>
                            <v-divider class="mt-4 border-0" />
                            <v-btn
                                variant="outlined"
                                color="default"
                                class="mr-2 mb-2"
                                :prepend-icon="Send"
                                :loading="isTesting"
                                @click="sendTest"
                            >
                                Send Test
                            </v-btn>
                            <v-btn
                                variant="outlined"
                                color="error"
                                class="mb-2"
                                :prepend-icon="Unlink"
                                :loading="isDisconnecting"
                                @click="disconnect"
                            >
                                Disconnect
                            </v-btn>
                        </template>

                        <template v-else-if="!isLoading">
                            <v-chip variant="tonal" size="small" class="font-weight-bold">
                                Not connected
                            </v-chip>
                            <p class="mt-4">
                                Open the link in Telegram and press Start. Only your confirmed nodes are
                                notified, so confirm your nodes on the Nodes page first.
                            </p>
                            <v-divider class="mt-4 border-0" />
                            <v-btn
                                variant="outlined"
                                color="default"
                                class="mr-2 mb-2"
                                :prepend-icon="Link"
                                :loading="isLinking"
                                @click="connect"
                            >
                                Connect Telegram
                            </v-btn>
                            <v-btn
                                v-if="hasOpenedLink"
                                variant="text"
                                color="default"
                                class="mb-2"
                                :prepend-icon="RefreshCw"
                                @click="fetchSettings"
                            >
                                I pressed Start
                            </v-btn>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { VAlert, VBtn, VCard, VCardText, VChip, VCol, VContainer, VDivider, VRow } from 'vuetify/components';
import { Link, RefreshCw, Send, Unlink } from '@lucide/vue';

import { NotificationsHttpAPI } from '@/extensions/notifications/api';
import { type TelegramSettings } from '@/extensions/notifications/types';
import { useNotify } from '@/composables/useNotify';
import { APIError } from '@/utils/error';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';

const api = new NotificationsHttpAPI();
const notify = useNotify();

const telegram = ref<TelegramSettings>({ enabled: false, botName: '', connected: false });
const isLoading = ref<boolean>(true);
const isForbidden = ref<boolean>(false);
const isLinking = ref<boolean>(false);
const isTesting = ref<boolean>(false);
const isDisconnecting = ref<boolean>(false);
const hasOpenedLink = ref<boolean>(false);

const botURL = computed<string>(() => `https://t.me/${encodeURIComponent(telegram.value.botName)}`);

async function fetchSettings(): Promise<void> {
    isLoading.value = true;
    try {
        telegram.value = (await api.get()).telegram;
    } catch (error) {
        // the backend only resolves nodes for verified accounts; that is a
        // state to explain, not an error to shout about.
        if (error instanceof APIError && error.status === 403) {
            isForbidden.value = true;
        } else {
            notify.notifyError(error as Error);
        }
    } finally {
        isLoading.value = false;
    }
}

/**
 * Opens the bot in Telegram. The chat is connected by the bot once the user
 * presses Start there, so there is nothing to wait for here; the user comes
 * back and refreshes.
 */
async function connect(): Promise<void> {
    // open the window before the request, so it is not blocked as a popup
    const opened = window.open('', '_blank');
    isLinking.value = true;
    try {
        const url = await api.createTelegramLink();
        if (opened) {
            opened.opener = null;
            opened.location.href = url;
        } else {
            window.location.href = url;
        }
        hasOpenedLink.value = true;
    } catch (error) {
        opened?.close();
        notify.notifyError(error as Error);
    } finally {
        isLinking.value = false;
    }
}

async function sendTest(): Promise<void> {
    isTesting.value = true;
    try {
        await api.testTelegram();
        notify.success('Test message sent. Check your Telegram chat.');
    } catch (error) {
        notify.notifyError(error as Error);
    } finally {
        isTesting.value = false;
    }
}

async function disconnect(): Promise<void> {
    isDisconnecting.value = true;
    try {
        await api.disconnectTelegram();
        hasOpenedLink.value = false;
        notify.success('Telegram disconnected.');
    } catch (error) {
        notify.notifyError(error as Error);
    } finally {
        isDisconnecting.value = false;
    }
    await fetchSettings();
}

onMounted(fetchSettings);
</script>
