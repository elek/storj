// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-8">
        Your encryption passphrase is ready to use.
        Now you can upload files into your buckets securely using an encryption passphrase only you know.
    </div>
</template>

<script setup lang="ts">
import { DialogStepComponent } from '@poc/types/common';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { PassphraseOption } from '@poc/types/managePassphrase';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { EdgeCredentials } from '@/types/accessGrants';

import Icon from '@/../static/images/accessGrants/newCreateFlow/accessCreated.svg';

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();

const props = defineProps<{
    passphrase: string;
    option: PassphraseOption;
}>();

defineExpose<DialogStepComponent>({
    title: 'Success',
    iconSrc: Icon,
    onEnter: () => {
        analyticsStore.eventTriggered(AnalyticsEvent.PASSPHRASE_CREATED, {
            method: props.option === PassphraseOption.EnterPassphrase ? 'enter' : 'generate',
        });

        bucketsStore.setEdgeCredentials(new EdgeCredentials());
        bucketsStore.setPassphrase(props.passphrase);
        bucketsStore.setPromptForPassphrase(false);
    },
});
</script>
