// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-7">
        <v-row>
            <v-col cols="12">
                <p>Confirm that the access details are correct before creating.</p>
                <v-list lines="one">
                    <v-list-item
                        v-for="item in items"
                        :key="item.title"
                        :title="item.title"
                        :subtitle="item.value"
                        class="pl-0"
                    />
                </v-list>
            </v-col>
        </v-row>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VRow, VCol, VList, VListItem } from 'vuetify/components';

import { Permission, AccessType } from '@/types/createAccessGrant';
import { AccessGrantEndDate } from '@poc/types/createAccessGrant';
import { DialogStepComponent } from '@poc/types/common';

interface Item {
    title: string;
    value: string;
}

const props = defineProps<{
    name: string;
    types: AccessType[];
    permissions: Permission[];
    buckets: string[];
    endDate: AccessGrantEndDate;
}>();

/**
 * Returns the data used to generate the info rows.
 */
const items = computed<Item[]>(() => {
    return [
        { title: 'Name', value: props.name },
        { title: 'Type', value: props.types.join(', ') },
        { title: 'Permissions', value: props.permissions.join(', ') },
        { title: 'Buckets', value: props.buckets.join(', ') || 'All buckets' },
        { title: 'End Date', value: props.endDate.title },
    ];
});

defineExpose<DialogStepComponent>({
    title: 'Confirm Details',
});
</script>
