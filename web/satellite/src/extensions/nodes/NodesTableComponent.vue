// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-text-field
        v-model="search"
        label="Search by node ID, wallet or address"
        :prepend-inner-icon="Search"
        single-line
        variant="solo-filled"
        flat
        hide-details
        clearable
        density="comfortable"
        class="mb-5"
    />

    <v-data-table
        :headers="headers"
        :items="nodes"
        :search="search"
        :loading="isLoading"
        :items-per-page-options="tableSizeOptions(nodes.length)"
        item-value="id"
        no-data-text="No nodes are registered to your email address."
    >
        <template #item.id="{ item }">
            <v-tooltip :text="item.id" location="top">
                <template #activator="{ props: tooltipProps }">
                    <span class="font-weight-bold text-no-wrap cursor-pointer" v-bind="tooltipProps" @click="() => copy(item.id)">
                        {{ shortNodeID(item.id) }}
                    </span>
                </template>
            </v-tooltip>
        </template>

        <template #item.status="{ item }">
            <v-chip :color="nodeStatusColor(nodeStatus(item))" size="small" variant="tonal">
                {{ nodeStatus(item) }}
            </v-chip>
        </template>

        <template #item.lastContactSuccess="{ item }">
            <span class="text-no-wrap">{{ formatContact(item.lastContactSuccess) }}</span>
        </template>

        <template #item.lastContactFailure="{ item }">
            <span class="text-no-wrap">{{ formatContact(item.lastContactFailure) }}</span>
        </template>

        <template #item.pieceCount="{ item }">
            {{ item.pieceCount.toLocaleString() }}
        </template>

        <template #item.freeDisk="{ item }">
            <span class="text-no-wrap">{{ formatDisk(item.freeDisk) }}</span>
        </template>

        <template #item.wallet="{ item }">
            <v-tooltip :text="item.wallet" location="top">
                <template #activator="{ props: tooltipProps }">
                    <span class="text-no-wrap cursor-pointer" v-bind="tooltipProps" @click="() => copy(item.wallet)">
                        {{ shortWallet(item.wallet) }}
                    </span>
                </template>
            </v-tooltip>
        </template>

        <template #item.vettedAt="{ item }">
            <span class="text-no-wrap">{{ item.vettedAt ? Time.formattedDate(item.vettedAt) : 'Not vetted' }}</span>
        </template>

        <template #item.confirmed="{ item }">
            <v-chip v-if="item.confirmed" color="success" size="small" variant="tonal">
                Confirmed
            </v-chip>
            <v-btn
                v-else
                size="small"
                variant="outlined"
                :loading="confirming === item.id"
                :disabled="confirming !== ''"
                @click="() => confirm(item)"
            >
                Confirm
            </v-btn>
        </template>
    </v-data-table>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VDataTable,
    VTextField,
    VChip,
    VTooltip,
    VBtn,
} from 'vuetify/components';
import { Search } from '@lucide/vue';

import { type Node, nodeStatus, nodeStatusColor } from '@/extensions/nodes/types';
import { NodesHttpAPI } from '@/extensions/nodes/api';
import { Time } from '@/utils/time';
import { Size } from '@/utils/bytesSize';
import { tableSizeOptions } from '@/types/common';
import { useNotify } from '@/composables/useNotify';

defineProps<{
    nodes: Node[];
    isLoading: boolean;
}>();

const emit = defineEmits<{
    confirmed: [nodeID: string];
}>();

const api = new NodesHttpAPI();
const notify = useNotify();

const search = ref<string>('');

/**
 * The id of the node whose confirmation is in flight, empty when idle.
 */
const confirming = ref<string>('');

const headers = [
    { title: 'Node ID', key: 'id' },
    { title: 'Status', key: 'status', sortable: false },
    { title: 'Last Contact', key: 'lastContactSuccess' },
    { title: 'Last Failure', key: 'lastContactFailure' },
    { title: 'Pieces', key: 'pieceCount' },
    { title: 'Free Disk', key: 'freeDisk' },
    { title: 'Address', key: 'lastIpPort' },
    { title: 'Wallet', key: 'wallet' },
    { title: 'Vetted', key: 'vettedAt' },
    { title: 'Version', key: 'version' },
    { title: 'Ownership', key: 'confirmed', sortable: false },
];

async function confirm(node: Node): Promise<void> {
    confirming.value = node.id;
    try {
        await api.confirm(node.id);
        emit('confirmed', node.id);
        notify.success('Node ownership confirmed');
    } catch (error) {
        notify.notifyError(error as Error);
    } finally {
        confirming.value = '';
    }
}

function shortNodeID(id: string): string {
    return id.length > 16 ? `${id.slice(0, 8)}…${id.slice(-6)}` : id;
}

function shortWallet(wallet: string): string {
    if (!wallet) return '-';
    return wallet.length > 14 ? `${wallet.slice(0, 6)}…${wallet.slice(-4)}` : wallet;
}

function formatDisk(bytes: number): string {
    // the node reports -1 when it has not told us yet.
    if (bytes < 0) return 'Unknown';
    const size = new Size(bytes, 2);
    return `${size.formattedBytes} ${size.label}`;
}

/**
 * A node that has never been contacted carries the zero time rather than a null,
 * which would otherwise render as a misleading date in the year 1.
 */
function formatContact(date: Date): string {
    if (isNaN(date.getTime()) || date.getUTCFullYear() <= 1) return 'Never';
    return Time.formattedDate(date, { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });
}

async function copy(value: string): Promise<void> {
    if (!value) return;
    await navigator.clipboard.writeText(value);
    notify.success('Copied to clipboard');
}
</script>
