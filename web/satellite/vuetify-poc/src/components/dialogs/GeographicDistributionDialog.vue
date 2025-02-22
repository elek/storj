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
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        variant="tonal"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <icon-distribution size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">
                    Geographic Distribution
                </v-card-title>
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

            <div class="pa-7">
                <img class="w-100" :src="mapURL" alt="map">
                <p class="font-weight-bold my-4">
                    You're getting this file from all over the world on the Storj network.
                </p>
                <p>
                    Storj Decentralized Cloud Storage splits files into smaller pieces, then distributes those pieces
                    over a global network of Nodes and recompiles them securely on download. This map shows the
                    real-time locations of this file’s pieces.
                </p>
            </div>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            @click="model = false"
                        >
                            Close
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            block
                            color="default"
                            link
                            href="https://docs.storj.io/learn#what-happens-when-you-upload"
                            target="_blank"
                            rel="noopener noreferrer"
                            append-icon="mdi-open-in-new"
                        >
                            Learn more
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VSheet,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
} from 'vuetify/components';

import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { PreviewCache, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

import IconDistribution from '@poc/components/icons/IconDistribution.vue';

const props = defineProps<{
    modelValue: boolean,
}>();

const emit = defineEmits<{
  'update:modelValue': [value: boolean];
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const analyticsStore = useAnalyticsStore();
const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();

const notify = useNotify();

/**
 * Returns bucket name from store.
 */
const bucket = computed<string>(() => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Retrieve the encoded filepath.
 */
const encodedFilePath = computed<string>(() => {
    return encodeURIComponent(`${bucket.value}/${obStore.state.objectPathForModal.trim()}`);
});

/**
 * Returns object preview URLs cache from store.
 */
const cachedObjectPreviewURLs = computed<Map<string, PreviewCache>>(() => {
    return obStore.state.cachedObjectPreviewURLs;
});

/**
 * Returns object map URL.
 */
const mapURL = computed<string>(() => {
    const cache = cachedObjectPreviewURLs.value.get(encodedFilePath.value);
    const url = cache?.url || '';
    return `${url}?map=1`;
});
</script>
