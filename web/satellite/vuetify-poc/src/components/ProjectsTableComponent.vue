// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" class="rounded-xlg">
        <v-text-field
            v-model="search"
            label="Search"
            prepend-inner-icon="mdi-magnify"
            single-line
            variant="solo-filled"
            flat
            hide-details
            clearable
            density="comfortable"
            rounded="lg"
            class="mx-2 mt-2"
        />

        <v-data-table
            :sort-by="sortBy"
            :headers="headers"
            :items="items"
            :search="search"
            no-data-text="No results found"
            item-key="path"
        >
            <template #item.name="{ item }">
                <v-btn
                    v-if="item.raw.role !== ProjectRole.Invited"
                    class="rounded-lg pl-1 pr-4 justify-start font-weight-bold"
                    variant="text"
                    height="40"
                    color="default"
                    block
                    @click="openProject(item.raw)"
                >
                    <img src="../assets/icon-project-tonal.svg" alt="Project" class="mr-3">
                    {{ item.raw.name }}
                </v-btn>
                <div v-else class="pl-1 pr-4 d-flex align-center justify-start font-weight-bold">
                    <img src="../assets/icon-project-tonal.svg" alt="Project" class="mr-3">
                    <span class="text-no-wrap">{{ item.raw.name }}</span>
                </div>
            </template>

            <template #item.role="{ item }">
                <v-chip :color="PROJECT_ROLE_COLORS[item.raw.role]" rounded="xl" size="small" class="font-weight-bold">
                    {{ item.raw.role }}
                </v-chip>
            </template>

            <template #item.createdAt="{ item }">
                {{ getFormattedDate(item.raw.createdAt) }}
            </template>

            <template #item.actions="{ item }">
                <div class="w-100 d-flex align-center justify-space-between">
                    <v-btn
                        v-if="item.raw.role === ProjectRole.Invited"
                        color="primary"
                        size="small"
                        :disabled="decliningIds.has(item.raw.id)"
                        @click="emit('joinClick', item.raw)"
                    >
                        Join Project
                    </v-btn>
                    <v-btn v-else color="primary" size="small" @click="openProject(item.raw)">Open Project</v-btn>

                    <v-btn
                        v-if="item.raw.role === ProjectRole.Owner || item.raw.role === ProjectRole.Invited"
                        class="ml-2"
                        icon
                        color="default"
                        variant="outlined"
                        size="small"
                        density="comfortable"
                        :loading="decliningIds.has(item.raw.id)"
                    >
                        <v-icon icon="mdi-dots-horizontal" size="18" />
                        <v-menu activator="parent" location="bottom" transition="scale-transition">
                            <v-list class="pa-1">
                                <template v-if="item.raw.role === ProjectRole.Owner">
                                    <v-list-item link @click="() => onSettingsClick(item.raw)">
                                        <template #prepend>
                                            <icon-settings />
                                        </template>
                                        <v-list-item-title class="text-body-2 ml-3">
                                            Project Settings
                                        </v-list-item-title>
                                    </v-list-item>
                                    <v-divider class="my-1" />
                                    <v-list-item link>
                                        <template #prepend>
                                            <icon-team size="18" />
                                        </template>
                                        <v-list-item-title class="text-body-2 ml-3">
                                            Invite Members
                                        </v-list-item-title>
                                    </v-list-item>
                                </template>
                                <v-list-item v-else link @click="declineInvitation(item.raw)">
                                    <template #prepend>
                                        <icon-trash />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        Decline
                                    </v-list-item-title>
                                </v-list-item>
                            </v-list>
                        </v-menu>
                    </v-btn>
                </div>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import {
    VCard,
    VTextField,
    VListItem,
    VChip,
    VBtn,
    VMenu,
    VList,
    VIcon,
    VListItemTitle,
    VDivider,
} from 'vuetify/components';
import { VDataTable } from 'vuetify/labs/components';

import { ProjectItemModel, PROJECT_ROLE_COLORS } from '@poc/types/projects';
import { ProjectInvitationResponse } from '@/types/projects';
import { ProjectRole } from '@/types/projectMembers';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';

import IconSettings from '@poc/components/icons/IconSettings.vue';
import IconTeam from '@poc/components/icons/IconTeam.vue';
import IconTrash from '@poc/components/icons/IconTrash.vue';

const props = defineProps<{
    items: ProjectItemModel[],
}>();

const emit = defineEmits<{
    (event: 'joinClick', item: ProjectItemModel): void;
}>();

const search = ref<string>('');
const decliningIds = ref(new Set<string>());

const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const router = useRouter();
const notify = useNotify();

const sortBy = [{ key: 'name', order: 'asc' }];
const headers = [
    { title: 'Project', key: 'name', align: 'start' },
    { title: 'Role', key: 'role' },
    { title: 'Members', key: 'memberCount' },
    { title: 'Date Added', key: 'createdAt' },
    { title: 'Actions', key: 'actions', sortable: false, width: '0' },
];

/**
 * Formats the given project creation date.
 */
function getFormattedDate(date: Date): string {
    return `${date.getDate()} ${SHORT_MONTHS_NAMES[date.getMonth()]} ${date.getFullYear()}`;
}

/**
 * Selects the project and navigates to the project dashboard.
 */
function openProject(item: ProjectItemModel): void {
    projectsStore.selectProject(item.id);
    router.push(`/projects/${projectsStore.state.selectedProject.urlId}/dashboard`);
    analyticsStore.pageVisit('/projects/dashboard');
}

/**
 * Selects the project and navigates to the project's settings.
 */
function onSettingsClick(item: ProjectItemModel): void {
    projectsStore.selectProject(item.id);
    router.push(`/projects/${projectsStore.state.selectedProject.urlId}/dashboard`);
}

/**
 * Declines the project invitation.
 */
async function declineInvitation(item: ProjectItemModel): Promise<void> {
    if (decliningIds.value.has(item.id)) return;
    decliningIds.value.add(item.id);

    try {
        await projectsStore.respondToInvitation(item.id, ProjectInvitationResponse.Decline);
        analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_INVITATION_DECLINED);
    } catch (error) {
        error.message = `Failed to decline project invitation. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_INVITATION);
    }

    try {
        await projectsStore.getUserInvitations();
        await projectsStore.getProjects();
    } catch (error) {
        error.message = `Failed to reload projects and invitations list. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_INVITATION);
    }

    decliningIds.value.delete(item.id);
}
</script>
