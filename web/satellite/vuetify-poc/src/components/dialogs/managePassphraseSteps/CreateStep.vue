// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-8">
        Creating a new passphrase allows you to upload data separately from
        the data uploaded with the current encryption passphrase.
    </div>
</template>

<script setup lang="ts">
import { DialogStepComponent } from '@poc/types/common';

defineExpose<DialogStepComponent>({
    title: 'Create a New Passphrase',
});
</script>
