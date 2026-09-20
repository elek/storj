// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Nodes" />
        <PageSubtitleComponent subtitle="The storage nodes registered with your email address." />

        <v-alert v-if="isForbidden" class="my-4" type="info" variant="tonal">
            Verify your email address to see the storage nodes registered to it.
        </v-alert>

        <v-alert v-else-if="page.truncated" class="my-4" type="info" variant="tonal">
            Showing the first {{ page.nodes.length.toLocaleString() }} nodes. You operate more than we can list here.
        </v-alert>

        <NodesTableComponent v-if="!isForbidden" :nodes="page.nodes" :is-loading="isLoading" @confirmed="onConfirmed" />
    </v-container>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { VContainer, VAlert } from 'vuetify/components';

import { NodesHttpAPI } from '@/extensions/nodes/api';
import { type NodesPage } from '@/extensions/nodes/types';
import { useNotify } from '@/composables/useNotify';
import { APIError } from '@/utils/error';
import NodesTableComponent from '@/extensions/nodes/NodesTableComponent.vue';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';

const api = new NodesHttpAPI();
const notify = useNotify();

const page = ref<NodesPage>({ nodes: [], truncated: false });
const isLoading = ref<boolean>(true);
const isForbidden = ref<boolean>(false);

/**
 * Re-reads the list rather than assuming the confirmation stuck, so that what
 * the table shows is always what the satellite actually recorded.
 */
async function onConfirmed(): Promise<void> {
    await fetchNodes();
}

async function fetchNodes(): Promise<void> {
    isLoading.value = true;
    try {
        page.value = await api.get();
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

onMounted(fetchNodes);
</script>
