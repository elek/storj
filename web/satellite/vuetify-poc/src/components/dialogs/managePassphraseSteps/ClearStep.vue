// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-8">
        By choosing to clear your passphrase for this session, your data will become locked
        while you can use the rest of the dashboard.
    </div>
</template>

<script setup lang="ts">
import { DialogStepComponent } from '@poc/types/common';
import { useNotify } from '@/utils/hooks';
import { useBucketsStore } from '@/store/modules/bucketsStore';

const bucketsStore = useBucketsStore();
const notify = useNotify();

defineExpose<DialogStepComponent>({
    title: 'Clear My Passphrase',
    onExit: to => {
        if (to !== 'next') return;
        bucketsStore.clearS3Data();
        notify.success('Passphrase cleared.');
    },
});
</script>
