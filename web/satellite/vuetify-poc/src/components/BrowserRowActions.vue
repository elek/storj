// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="text-no-wrap" :class="alignClass">
        <v-btn
            v-if="file.type !== 'folder'"
            variant="outlined"
            color="default"
            size="small"
            class="mr-1 text-caption"
            density="comfortable"
            icon
            :loading="isDownloading"
            @click="onDownloadClick"
        >
            <icon-download />
            <v-tooltip activator="parent" location="start">Download</v-tooltip>
        </v-btn>

        <v-btn
            variant="outlined"
            color="default"
            size="small"
            class="mr-1 text-caption"
            density="comfortable"
            icon
        >
            <v-icon icon="mdi-dots-horizontal" />
            <v-menu activator="parent">
                <v-list class="pa-1">
                    <template v-if="file.type !== 'folder'">
                        <v-list-item density="comfortable" link rounded="lg" @click="emit('previewClick')">
                            <template #prepend>
                                <icon-preview />
                            </template>
                            <v-list-item-title class="pl-2 ml-2 text-body-2 font-weight-medium">
                                Preview
                            </v-list-item-title>
                        </v-list-item>

                        <v-list-item
                            density="comfortable"
                            :link="!isDownloading"
                            rounded="lg"
                            @click="onDownloadClick"
                        >
                            <template #prepend>
                                <icon-download />
                            </template>
                            <v-fade-transition>
                                <v-list-item-title v-show="!isDownloading" class="pl-2 ml-2 text-body-2 font-weight-medium">
                                    Download
                                </v-list-item-title>
                            </v-fade-transition>
                            <div v-if="isDownloading" class="browser_actions_menu__loader">
                                <v-progress-circular indeterminate size="23" width="2" />
                            </div>
                        </v-list-item>
                    </template>

                    <v-list-item density="comfortable" link rounded="lg" @click="emit('shareClick')">
                        <template #prepend>
                            <icon-share bold />
                        </template>
                        <v-list-item-title class="pl-2 ml-2 text-body-2 font-weight-medium">
                            Share
                        </v-list-item-title>
                    </v-list-item>

                    <v-divider class="my-1" />

                    <v-list-item density="comfortable" link rounded="lg" base-color="error" @click="emit('deleteFileClick')">
                        <template #prepend>
                            <icon-trash bold />
                        </template>
                        <v-list-item-title class="pl-2 ml-2 text-body-2 font-weight-medium">
                            Delete
                        </v-list-item-title>
                    </v-list-item>
                </v-list>
            </v-menu>
        </v-btn>
    </div>
</template>

<script setup lang="ts">
import { ref, h, computed } from 'vue';
import {
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VDivider,
    VProgressCircular,
    VFadeTransition,
    VIcon,
    VBtn,
    VTooltip,
} from 'vuetify/components';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import IconDownload from '@poc/components/icons/IconDownload.vue';
import IconShare from '@poc/components/icons/IconShare.vue';
import IconPreview from '@poc/components/icons/IconPreview.vue';
import IconTrash from '@poc/components/icons/IconTrash.vue';

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();
const notify = useNotify();

const props = defineProps<{
    file: BrowserObject;
    align: 'left' | 'right';
}>();

const emit = defineEmits<{
    previewClick: [];
    deleteFileClick: [];
    shareClick: [];
}>();

const isDownloading = ref<boolean>(false);

const alignClass = computed<string>(() => {
    return 'text-' + props.align;
});

async function onDownloadClick(): Promise<void> {
    if (isDownloading.value) {
        return;
    }

    isDownloading.value = true;
    try {
        await obStore.download(props.file);
        notify.success(
            () => ['Keep this download link private.', h('br'), 'If you want to share, use the Share option.'],
            'Download Started',
        );
    } catch (error) {
        error.message = `Error downloading file. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.FILE_BROWSER_ENTRY);
    }
    isDownloading.value = false;
}
</script>

<style scoped lang="scss">
.browser_actions_menu__loader {
    inset: 0;
    position: absolute;
    display: flex;
    align-items: center;
    justify-content: center;
}
</style>
