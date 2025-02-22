// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr
        class="item-container"
        :class="{ 'selected': selected }"
        @click="onClick"
    >
        <th v-if="selectable" class="icon select" @click.stop="selectClicked">
            <v-table-checkbox v-if="!selectHidden" :disabled="selectDisabled || selectHidden" :value="selected" @selectClicked="selectClicked" />
        </th>
        <th
            v-for="(val, keyVal, index) in item" :key="index" class="align-left data"
            :class="{'overflow-visible': showBucketGuide(index)}"
        >
            <div v-if="Array.isArray(val)" class="few-items-container">
                <div v-if="icon && index === 0 && itemType?.includes('project')" class="item-icon file-background" :class="customIconClasses">
                    <component :is="icon" />
                </div>
                <div class="few-items">
                    <p v-for="str in val" :key="str" class="array-val">{{ str }}</p>
                </div>
            </div>
            <div v-else class="table-item">
                <div v-if="icon && index === 0" class="item-icon file-background" :class="customIconClasses">
                    <component :is="icon" />
                </div>
                <p v-if="keyVal === 'multi'" class="multi" :class="{primary: index === 0}" :title="val['title']" @click.stop="(e) => cellContentClicked(index, e)">
                    <span class="multi__title">{{ val['title'] }}</span>
                    <span class="multi__subtitle">{{ val['subtitle'] }}</span>
                </p>
                <p v-else :class="{primary: index === 0}" :title="val" @click.stop="(e) => cellContentClicked(index, e)">
                    <middle-truncate v-if="keyVal === 'fileName'" :text="val" />
                    <project-ownership-tag v-else-if="keyVal === 'role'" :no-icon="!isProjectRoleIconShown(val)" :role="val" />
                    <span v-else>{{ val }}</span>
                </p>
                <div v-if="showBucketGuide(index)" class="animation">
                    <span><span /></span>
                    <BucketGuide :hide-guide="hideGuide" />
                </div>
            </div>
        </th>
        <slot name="options" />
    </tr>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { ProjectRole } from '@/types/projectMembers';
import { ObjectType } from '@/utils/objectIcon';

import VTableCheckbox from '@/components/common/VTableCheckbox.vue';
import BucketGuide from '@/components/objects/BucketGuide.vue';
import MiddleTruncate from '@/components/browser/MiddleTruncate.vue';
import ProjectOwnershipTag from '@/components/project/ProjectOwnershipTag.vue';

const props = withDefaults(defineProps<{
    selectDisabled?: boolean;
    selectHidden?: boolean;
    selected?: boolean;
    selectable?: boolean;
    showGuide?: boolean;
    itemType?: string;
    item?: object;
    onClick?: (data?: unknown) => void;
    hideGuide?: () => void;
    // event for the first cell of this item.
    onPrimaryClick?: (data?: unknown) => void;
}>(), {
    selectDisabled: false,
    selectHidden: false,
    selected: false,
    selectable: false,
    showGuide: false,
    itemType: 'none',
    item: () => ({}),
    onClick: () => {},
    hideGuide: () => {},
    onPrimaryClick: undefined,
});

const emit = defineEmits(['selectClicked']);

const icon = computed((): string => ObjectType.findIcon(props.itemType));

const customIconClasses = computed(() => {
    const classes = {};
    if (props.itemType === 'project') {
        classes['project-owner'] = true;
    } else if (props.itemType === 'shared-project') {
        classes['project-member'] = true;
    }
    return classes;
});

function isProjectRoleIconShown(role: ProjectRole) {
    return props.itemType.includes('project') || role === ProjectRole.Invited || role === ProjectRole.InviteExpired;
}

function selectClicked(event: Event): void {
    emit('selectClicked', event);
}

function showBucketGuide(index: number): boolean {
    return (props.itemType?.toLowerCase() === 'bucket') && (index === 0) && props.showGuide;
}

function cellContentClicked(cellIndex: number, event: Event) {
    if (cellIndex === 0 && props.onPrimaryClick) {
        props.onPrimaryClick(event);
        return;
    }
    // trigger default item onClick instead.
    if (props.onClick) {
        props.onClick();
    }
}
</script>

<style scoped lang="scss">
    @mixin keyframes() {
        @keyframes pulse {

            0% {
                opacity: 0.75;
                transform: scale(1);
            }

            25% {
                opacity: 0.75;
                transform: scale(1);
            }

            100% {
                opacity: 0;
                transform: scale(2.5);
            }
        }
    }

    @include keyframes;

    .animation {
        border-radius: 50%;
        height: 8px;
        width: 8px;
        margin-left: 23px;
        margin-top: 5px;
        background-color: #0149ff;
        position: relative;

        > span {
            animation: pulse 1s linear infinite;
            border-radius: 50%;
            display: block;
            height: 8px;
            width: 8px;

            > span {
                animation: pulse 1s linear infinite;
                border-radius: 50%;
                display: block;
                height: 8px;
                width: 8px;
            }
        }

        span {
            background-color: rgb(1 73 255 / 2000%);

            &:after {
                background-color: rgb(1 73 255 / 2000%);
            }
        }
    }

    tr {
        cursor: pointer;

        &:hover {
            background: var(--c-grey-1);

            .table-item {

                .primary {
                    color: var(--c-blue-3);

                    & > .multi__subtitle {
                        color: var(--c-blue-3);
                    }
                }
            }
        }

        &.selected {
            background: var(--c-yellow-1);

            :deep(.select) {
                background: var(--c-yellow-1);
            }
        }
    }

    .multi {
        display: flex;
        flex-direction: column;

        &__title {
            text-overflow: ellipsis;
            overflow: hidden;
        }

        &__subtitle {
            font-family: 'font_regular', sans-serif;
            font-size: 12px;
            line-height: 20px;
            color: var(--c-grey-6);
            text-overflow: ellipsis;
            overflow: hidden;
        }
    }

    .few-items-container {
        display: flex;
        align-items: center;

        @media screen and (width <= 370px) {
            max-width: 9rem;
        }
    }

    .few-items {
        display: flex;
        flex-direction: column;
        justify-content: space-between;
    }

    .array-val {
        font-family: 'font_regular', sans-serif;
        font-size: 0.75rem;
        line-height: 1.25rem;

        &:first-of-type {
            font-family: 'font_bold', sans-serif;
            font-size: 0.875rem;
            margin-bottom: 3px;
        }
    }

    .table-item {
        display: flex;
        align-items: center;
    }

    .item-container {
        position: relative;
    }

    .item-icon {
        margin-right: 12px;
        min-width: 18px;
    }

    .file-background {
        background: var(--c-white);
        border: 1px solid var(--c-grey-2);
        padding: 2px;
        border-radius: 8px;
        height: 32px;
        min-width: 32px;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .project-owner {

        :deep(path) {
            fill: var(--c-purple-4);
        }
    }

    .project-member {

        :deep(path) {
            fill: var(--c-yellow-5);
        }
    }
</style>
