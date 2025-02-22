// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-area">
        <HeaderArea
            class="team-area__header"
            :selected-project-members-count="selectedProjectMembersLength"
            :is-add-button-disabled="areMembersFetching"
        />
        <VLoader v-if="areMembersFetching" width="100px" height="100px" />

        <div v-if="isEmptySearchResultShown && !areMembersFetching" class="team-area__empty-search-result-area">
            <h1 class="team-area__empty-search-result-area__title">No results found</h1>
            <EmptySearchResultIcon class="team-area__empty-search-result-area__image" />
        </div>

        <v-table
            v-if="!areMembersFetching && !isEmptySearchResultShown"
            items-label="project members"
            :selectable="true"
            :limit="projectMemberLimit"
            :total-page-count="totalPageCount"
            :total-items-count="projectMembersTotalCount"
            :on-page-change="onPageChange"
        >
            <template #head>
                <th class="align-left">Name</th>
                <th class="align-left">Email</th>
                <th class="align-left">Role</th>
                <th class="align-left date-added">Date Added</th>
            </template>
            <template #body>
                <ProjectMemberListItem
                    v-for="(member, key) in projectMembers"
                    :key="key"
                    :model="member"
                    @removeClick="onRemoveClick"
                    @resendClick="onResendClick"
                    @memberClick="onMemberCheckChange"
                    @selectClick="onMemberCheckChange"
                />
            </template>
        </v-table>
    </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue';

import { ProjectMemberItemModel } from '@/types/projectMembers';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useLoading } from '@/composables/useLoading';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';

import VLoader from '@/components/common/VLoader.vue';
import HeaderArea from '@/components/team/HeaderArea.vue';
import ProjectMemberListItem from '@/components/team/ProjectMemberListItem.vue';
import VTable from '@/components/common/VTable.vue';

import EmptySearchResultIcon from '@/../static/images/common/emptySearchResult.svg';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const pmStore = useProjectMembersStore();
const projectsStore = useProjectsStore();
const configStore = useConfigStore();
const notify = useNotify();

const { withLoading } = useLoading();

const FIRST_PAGE = 1;

const areMembersFetching = ref<boolean>(true);

/**
 * Returns team members of current page from store.
 * With project owner pinned to top
 */
const projectMembers = computed((): ProjectMemberItemModel[] => {
    const projectMembers = pmStore.state.page.getAllItems();
    const projectOwner = projectMembers.find((member) => member.getUserID() === projectsStore.state.selectedProject.ownerId);
    const projectMembersToReturn = projectMembers.filter((member) => member.getUserID() !== projectsStore.state.selectedProject.ownerId);

    // if the project owner exists, place at the front of the members list
    projectOwner && projectMembersToReturn.unshift(projectOwner);

    return projectMembersToReturn;
});

/**
 * Returns team members total page count from store.
 */
const projectMembersTotalCount = computed((): number => {
    return pmStore.state.page.totalCount;
});

/**
 * Returns team members limit from store.
 */
const projectMemberLimit = computed((): number => {
    return pmStore.state.page.limit;
});

/**
 * Returns team members count of current page from store.
 */
const projectMembersCount = computed((): number => {
    return pmStore.state.page.projectMembers.length;
});

const totalPageCount = computed((): number => {
    return pmStore.state.page.pageCount;
});

const selectedProjectMembersLength = computed((): number => {
    return pmStore.state.selectedProjectMembersEmails.length;
});

const isEmptySearchResultShown = computed((): boolean => {
    return projectMembersCount.value === 0 && projectMembersTotalCount.value === 0;
});

/**
 * Selects team member if this user has no owner status.
 * @param member
 */
function onMemberCheckChange(member: ProjectMemberItemModel): void {
    if (projectsStore.state.selectedProject.ownerId !== member.getUserID()) {
        pmStore.toggleProjectMemberSelection(member);
    }
}

/**
 * Fetches team member of selected page.
 * @param index
 * @param limit
 */
async function onPageChange(index: number, limit: number): Promise<void> {
    try {
        await pmStore.getProjectMembers(index, projectsStore.state.selectedProject.id, limit);
    } catch (error) {
        notify.error(`Unable to fetch project members. ${error.message}`, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
    }
}

function onResendClick(member: ProjectMemberItemModel) {
    withLoading(async () => {
        analyticsStore.eventTriggered(AnalyticsEvent.RESEND_INVITE_CLICKED);
        try {
            await pmStore.reinviteMembers([member.getEmail()], projectsStore.state.selectedProject.id);

            if (configStore.state.config.unregisteredInviteEmailsEnabled) {
                notify.success('Invite re-sent!');
            } else {
                notify.success(() => [
                    h('p', { class: 'message-title' }, 'Invites re-sent!'),
                    h('p', { class: 'message-info' }, [
                        'The invitation will be re-sent to the email address if it belongs to a user on this satellite.',
                    ]),
                ]);
            }

            pmStore.setSearchQuery('');
        } catch (error) {
            error.message = `Error resending invite. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
            return;
        }

        try {
            await pmStore.getProjectMembers(FIRST_PAGE, projectsStore.state.selectedProject.id);
        } catch (error) {
            notify.error(`Unable to fetch project members. ${error.message}`, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
        }
    });
}

async function onRemoveClick(member: ProjectMemberItemModel) {
    if (projectsStore.state.selectedProject.ownerId !== member.getUserID()) {
        if (!member.isSelected()) {
            pmStore.toggleProjectMemberSelection(member);
        }
        appStore.updateActiveModal(MODALS.removeTeamMember);
    }
    analyticsStore.eventTriggered(AnalyticsEvent.REMOVE_PROJECT_MEMBER_CLICKED);
}

/**
 * Lifecycle hook after initial render.
 * Fetches first page of team members list of current project.
 */
onMounted(async (): Promise<void> => {
    try {
        await pmStore.getProjectMembers(FIRST_PAGE, projectsStore.state.selectedProject.id);

        areMembersFetching.value = false;
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
    }
});
</script>

<style scoped lang="scss">
    .team-area {
        padding: 40px 30px 55px;
        font-family: 'font_regular', sans-serif;

        &__header {
            width: 100%;
            margin-bottom: 20px;
            background-color: #f5f6fa;
            top: auto;
        }

        &__empty-search-result-area {
            height: 100%;
            display: flex;
            align-items: center;
            justify-content: center;
            flex-direction: column;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
            }

            &__image {
                margin-top: 40px;
            }
        }
    }

    @media screen and (width <= 800px) and (width >= 500px) {

        .date-added {
            display: none;
        }
    }
</style>
