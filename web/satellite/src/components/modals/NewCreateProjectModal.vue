// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <blue-box-icon />
                    <span class="modal__header__title">Create new project</span>
                </div>

                <div class="modal__divider" />

                <p class="modal__info">
                    Projects are where you and your team can upload and manage data, view
                    usage statistics and billing.
                </p>

                <div class="modal__divider" />

                <VInput
                    label="Project Name"
                    additional-label="Required. Up to 20 characters"
                    placeholder="Enter Project Name"
                    :max-symbols="20"
                    :error="nameError"
                    @setData="setProjectName"
                />

                <div class="modal__divider" />

                <VInput
                    v-if="hasDescription"
                    label="Project Description"
                    placeholder="Enter Project Description"
                    additional-label="Optional. Up to 50 characters"
                    :max-symbols="50"
                    is-multiline
                    height="100px"
                    @setData="setProjectDescription"
                />
                <div v-else class="modal__project-description">
                    <p class="modal__project-description__label">Project Description</p>
                    <a
                        class="modal__project-description__action" href=""
                        @click.prevent="hasDescription = !hasDescription"
                    >Add Description (optional)</a>
                </div>

                <div class="modal__divider" />

                <div class="modal__button-container">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="48px"
                        :on-press="closeModal"
                        :is-transparent="true"
                    />
                    <VButton
                        label="Create Project -->"
                        width="100%"
                        height="48px"
                        :on-press="onCreateProjectClick"
                        :is-disabled="!projectName"
                    />
                </div>
                <div v-if="isLoading" class="modal__blur">
                    <VLoader class="modal__blur__loader" width="50px" height="50px" />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { ProjectFields } from '@/types/projects';
import { LocalData } from '@/utils/localData';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VLoader from '@/components/common/VLoader.vue';
import VInput from '@/components/common/VInput.vue';
import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';

import BlueBoxIcon from '@/../static/images/common/blueBox.svg';

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const pmStore = useProjectMembersStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const router = useRouter();
const notify = useNotify();

const description = ref('');
const createdProjectId = ref('');
const hasDescription = ref(false);
const isLoading = ref(false);
const projectName = ref('');
const nameError = ref('');

/**
 * Sets project name from input value.
 */
function setProjectName(value: string): void {
    projectName.value = value;
    nameError.value = '';
}

/**
 * Sets project description from input value.
 */
function setProjectDescription(value: string): void {
    description.value = value;
}

/**
 * Creates project and refreshes store.
 */
async function onCreateProjectClick(): Promise<void> {
    if (isLoading.value) {
        return;
    }

    isLoading.value = true;
    projectName.value = projectName.value.trim();

    const project = new ProjectFields(
        projectName.value,
        description.value,
        usersStore.state.user.id,
    );

    try {
        project.checkName();
    } catch (error) {
        isLoading.value = false;
        nameError.value = error.message;
        analyticsStore.errorEventTriggered(AnalyticsErrorEventSource.CREATE_PROJECT_MODAL);

        return;
    }

    try {
        const newProj = await projectsStore.createProject(project);
        createdProjectId.value = newProj.id;
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.CREATE_PROJECT_MODAL);
        isLoading.value = false;

        return;
    }

    await selectCreatedProject();

    notify.success('Project created successfully!');

    isLoading.value = false;
    closeModal();

    bucketsStore.clearS3Data();

    if (usersStore.shouldOnboard) {
        analyticsStore.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
        await router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
        return;
    }

    analyticsStore.pageVisit(RouteConfig.ProjectDashboard.path);
    await router.push(RouteConfig.ProjectDashboard.path);

    if (usersStore.state.settings.passphrasePrompt) {
        appStore.updateActiveModal(MODALS.enterPassphrase);
    }
}

/**
 * Selects just created project.
 */
async function selectCreatedProject() {
    projectsStore.selectProject(createdProjectId.value);
    LocalData.setSelectedProjectId(createdProjectId.value);
    pmStore.setSearchQuery('');

    bucketsStore.clearS3Data();
}

/**
 * Closes create project modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}
</script>

<style scoped lang="scss">
.modal {
    width: 400px;
    padding: 54px 48px 51px;
    display: flex;
    flex-direction: column;
    font-family: 'font_regular', sans-serif;

    &__header {
        display: flex;
        align-items: center;
        gap: 16px;

        &__title {
            font-family: 'font-medium', sans-serif;
            font-weight: bold;
            font-size: 24px;
            line-height: 31px;
        }
    }

    &__divider {
        margin: 20px 0;
        border: 0.5px solid var(--c-grey-2);
    }

    &__project-description {
        font-family: 'font_regular', sans-serif;
        text-align: start;

        &__label {
            font-weight: bold;
            font-size: 16px;
            line-height: 21px;
            color: #354049;
        }

        &__action {
            font-size: 14px;
            line-height: 22px;
            text-decoration: underline;
            color: #354049;
        }
    }

    @media screen and (width <= 550px) {
        width: calc(100% - 48px);
        padding: 54px 24px 32px;
    }

    &__info {
        font-family: 'font_regular', sans-serif;
        text-align: start;
        font-size: 16px;
        line-height: 21px;
        color: #354049;
    }

    &__button-container {
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: space-between;
        column-gap: 20px;

        @media screen and (width <= 550px) {
            column-gap: unset;
            row-gap: 8px;
            flex-direction: column-reverse;
        }
    }

    &__blur {
        position: absolute;
        top: 0;
        left: 0;
        height: 100%;
        width: 100%;
        background-color: rgb(229 229 229 / 20%);
        border-radius: 8px;
        z-index: 100;

        &__loader {
            width: 25px;
            height: 25px;
            position: absolute;
            right: 40px;
            top: 40px;
        }
    }
}

.full-input {
    margin-top: 20px;
}

@media screen and (width <= 550px) {

    :deep(.add-label) {
        display: none;
    }
}
</style>
