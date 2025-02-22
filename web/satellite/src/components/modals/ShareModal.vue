// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <ModalHeader
                    :icon="ShareIcon"
                    :title="'Share ' + shareType"
                />
                <div class="modal__notification">
                    <p class="modal__notification__msg">
                        Sharing a {{ shareType.toLowerCase() }} will create a publicly shareable URL.
                        Anyone with this link will be able to access your shared {{ shareType.toLowerCase() }}.
                    </p>
                </div>
                <VLoader v-if="loading" width="40px" height="40px" />
                <template v-else>
                    <h1 class="modal__title">Share via</h1>
                    <ShareContainer :link="link" />
                    <label for="url" class="modal__label">Copy link</label>
                    <input
                        id="url"
                        class="modal__input"
                        type="url"
                        :value="link"
                        readonly
                    >
                </template>
                <VButton
                    class="modal__button"
                    :label="copyButtonState === ButtonStates.Copy ? 'Copy link' : 'Link Copied'"
                    height="52px"
                    width="100%"
                    border-radius="10px"
                    font-size="14px"
                    :icon="copyButtonState === ButtonStates.Copy ? 'none' : 'check'"
                    :on-press="onCopy"
                    :is-green="copyButtonState === ButtonStates.Copied"
                />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { useAppStore } from '@/store/modules/appStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useLinksharing } from '@/composables/useLinksharing';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { ShareType } from '@/types/browser';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import VModal from '@/components/common/VModal.vue';
import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';
import ShareContainer from '@/components/common/share/ShareContainer.vue';
import ModalHeader from '@/components/modals/ModalHeader.vue';

import ShareIcon from '@/../static/images/browser/galleryView/modals/share.svg';

enum ButtonStates {
    Copy,
    Copied,
}

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();

const { generateFileOrFolderShareURL, generateBucketShareURL } = useLinksharing();
const notify = useNotify();

const link = ref<string>('');
const loading = ref<boolean>(true);
const copyButtonState = ref<ButtonStates>(ButtonStates.Copy);

/**
 * Returns what type of entity is being shared.
 */
const shareType = computed((): ShareType => {
    return appStore.state.shareModalType;
});

/**
 * Retrieve the path to the current file.
 */
const filePath = computed((): string => {
    return obStore.state.objectPathForModal;
});

/**
 * Copies link to user's clipboard.
 */
async function onCopy(): Promise<void> {
    await navigator.clipboard.writeText(link.value);
    copyButtonState.value = ButtonStates.Copied;

    setTimeout(() => {
        copyButtonState.value = ButtonStates.Copy;
    }, 2000);
}

/**
 * Closes the modal.
 */
function closeModal(): void {
    if (loading.value) return;

    appStore.removeActiveModal();
}

onMounted(async (): Promise<void> => {
    analyticsStore.eventTriggered(AnalyticsEvent.LINK_SHARED);
    try {
        if (shareType.value === ShareType.Bucket) {
            link.value = await generateBucketShareURL(bucketsStore.state.fileComponentBucketName);
        } else {
            link.value = await generateFileOrFolderShareURL(
                bucketsStore.state.fileComponentBucketName,
                filePath.value,
                shareType.value === ShareType.Folder,
            );
        }
    } catch (error) {
        error.message = `Unable to get sharing URL. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.SHARE_MODAL);
    }

    loading.value = false;
});
</script>

<style scoped lang="scss">
.modal {
    padding: 32px;
    font-family: 'font_regular', sans-serif;
    background-color: var(--c-white);
    box-shadow: 0 20px 30px rgb(10 27 44 / 20%);
    border-radius: 20px;
    width: 410px;
    box-sizing: border-box;

    @media screen and (width <= 520px) {
        width: unset;
    }

    &__notification {
        margin-bottom: 16px;
        border-radius: 10px;
        border: 1px solid var(--c-yellow-2);
        background: var(--c-yellow-1);
        padding: 16px;

        &__msg {
            font-family: 'font-medium', sans-serif;
            font-size: 14px;
            line-height: 20px;
            text-align: left;
        }
    }

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-blue-6);
        margin-bottom: 16px;
        text-align: left;
    }

    &__label {
        display: block;
        margin: 16px 0 4px;
        padding-top: 16px;
        border-top: 1px solid var(--c-grey-2);
        text-align: left;
        font-family: 'font_bold', sans-serif;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-blue-6);
    }

    &__input {
        background: var(--c-white);
        border: 1px solid var(--c-grey-4);
        color: var(--c-grey-6);
        outline: none;
        max-width: 100%;
        width: 100%;
        padding: 9px 0 9px 13px;
        box-sizing: border-box;
        border-radius: 6px;
    }

    &__button {
        margin-top: 16px;
    }
}
</style>
