// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="navigation-area">
        <div class="navigation-area__container">
            <header class="navigation-area__container__header">
                <LogoIcon class="navigation-area__container__header__logo" @click.stop="onLogoClick" />
                <CrossIcon v-if="isOpened" @click="toggleNavigation" />
                <MenuIcon v-else @click="toggleNavigation" />
            </header>
            <div v-if="isOpened" class="navigation-area__container__wrap" :class="{ 'with-padding': isAccountDropdownShown }">
                <div class="navigation-area__container__wrap__edit">
                    <div
                        class="project-selection__selected"
                        aria-roledescription="project-selection"
                        @click.stop.prevent="onProjectClick"
                    >
                        <div class="project-selection__selected__left">
                            <ProjectIcon class="project-selection__selected__left__image" />
                            <p class="project-selection__selected__left__name" :title="selectedProject.name">{{ selectedProject.name }}</p>
                            <p class="project-selection__selected__left__placeholder">Projects</p>
                        </div>
                        <ArrowIcon class="project-selection__selected__arrow" />
                    </div>
                    <div v-if="isProjectDropdownShown" class="project-selection__dropdown">
                        <div v-if="isLoading" class="project-selection__dropdown__loader-container">
                            <VLoader width="30px" height="30px" />
                        </div>
                        <template v-else>
                            <div v-if="ownProjects.length" class="project-selection__dropdown__section-head">
                                <ProjectIcon />
                                <span class="project-selection__dropdown__section-head__tag">My Projects</span>
                            </div>
                            <div class="project-selection__dropdown__items">
                                <div
                                    v-for="project in ownProjects"
                                    :key="project.id"
                                    class="project-selection__dropdown__items__choice"
                                    @click.prevent.stop="onProjectSelected(project.id)"
                                    @keyup.enter="onProjectSelected(project.id)"
                                >
                                    <div v-if="project.isSelected" class="project-selection__dropdown__items__choice__mark-container">
                                        <CheckmarkIcon class="project-selection__dropdown__items__choice__mark-container__image" />
                                    </div>
                                    <p
                                        :class="{
                                            'project-selection__dropdown__items__choice__unselected': !project.isSelected,
                                            'project-selection__dropdown__items__choice__selected': project.isSelected,
                                        }"
                                    >
                                        {{ project.name }}
                                    </p>
                                </div>
                            </div>

                            <div v-if="sharedProjects.length" class="project-selection__dropdown__section-head shared">
                                <ProjectIcon />
                                <span class="project-selection__dropdown__section-head__tag shared">Shared with me</span>
                            </div>
                            <div class="project-selection__dropdown__items">
                                <div
                                    v-for="project in sharedProjects"
                                    :key="project.id"
                                    class="project-selection__dropdown__items__choice"
                                    @click.prevent.stop="onProjectSelected(project.id)"
                                    @keyup.enter="onProjectSelected(project.id)"
                                >
                                    <div v-if="project.isSelected" class="project-selection__dropdown__items__choice__mark-container">
                                        <CheckmarkIcon class="project-selection__dropdown__items__choice__mark-container__image" />
                                    </div>
                                    <p
                                        :class="{
                                            'project-selection__dropdown__items__choice__unselected': !project.isSelected,
                                            'project-selection__dropdown__items__choice__selected': project.isSelected,
                                        }"
                                    >
                                        {{ project.name }}
                                    </p>
                                </div>
                            </div>
                        </template>
                        <div v-if="isProjectOwner" tabindex="0" class="project-selection__dropdown__link-container" @click.stop="onProjectDetailsClick" @keyup.enter="onProjectDetailsClick">
                            <SettingsIcon />
                            <p class="project-selection__dropdown__link-container__label">Project Settings</p>
                        </div>
                        <div tabindex="0" class="project-selection__dropdown__link-container" @click.stop="onAllProjectsClick" @keyup.enter="onAllProjectsClick">
                            <ProjectIcon />
                            <p class="project-selection__dropdown__link-container__label">All projects</p>
                        </div>
                        <div tabindex="0" class="project-selection__dropdown__link-container" @click.stop="onManagePassphraseClick" @keyup.enter="onManagePassphraseClick">
                            <PassphraseIcon />
                            <p class="project-selection__dropdown__link-container__label">Manage Passphrase</p>
                        </div>
                        <div class="project-selection__dropdown__link-container" @click.stop="onCreateLinkClick">
                            <CreateProjectIcon />
                            <p class="project-selection__dropdown__link-container__label">Create new</p>
                        </div>
                    </div>
                </div>
                <div v-if="!isProjectDropdownShown" class="navigation-area__container__wrap__border" />
                <router-link
                    v-for="navItem in navigation"
                    :key="navItem.name"
                    :aria-label="navItem.name"
                    class="navigation-area__container__wrap__item-container"
                    :to="navItem.path"
                    @click.native="onNavClick(navItem.path)"
                >
                    <div class="navigation-area__container__wrap__item-container__left">
                        <component :is="navItem.icon" class="navigation-area__container__wrap__item-container__left__image" />
                        <p class="navigation-area__container__wrap__item-container__left__label">{{ navItem.name }}</p>
                    </div>
                </router-link>
                <div class="navigation-area__container__wrap__border" />
                <div class="container-wrapper">
                    <div
                        class="navigation-area__container__wrap__item-container"
                        @click.stop="toggleResourcesDropdown"
                    >
                        <div class="navigation-area__container__wrap__item-container__left">
                            <ResourcesIcon class="navigation-area__container__wrap__item-container__left__image" />
                            <p class="navigation-area__container__wrap__item-container__left__label">Resources</p>
                        </div>
                        <ArrowIcon class="navigation-area__container__wrap__item-container__arrow" />
                    </div>
                    <div
                        v-if="isResourcesDropdownShown"
                    >
                        <ResourcesLinks />
                    </div>
                </div>
                <div class="container-wrapper">
                    <div
                        class="navigation-area__container__wrap__item-container"
                        @click.stop="toggleQuickStartDropdown"
                    >
                        <div class="navigation-area__container__wrap__item-container__left">
                            <QuickStartIcon class="navigation-area__container__wrap__item-container__left__image" />
                            <p class="navigation-area__container__wrap__item-container__left__label">Quickstart</p>
                        </div>
                        <ArrowIcon class="navigation-area__container__wrap__item-container__arrow" />
                    </div>
                    <div
                        v-if="isQuickStartDropdownShown"
                    >
                        <QuickStartLinks />
                    </div>
                </div>
                <div class="account-area">
                    <div class="account-area__wrap" aria-roledescription="account-area" @click.stop="toggleAccountDropdown">
                        <div class="account-area__wrap__left">
                            <AccountIcon class="account-area__wrap__left__icon" />
                            <p class="account-area__wrap__left__label">My Account</p>
                            <p class="account-area__wrap__left__label-small">Account</p>
                            <template v-if="billingEnabled">
                                <TierBadgePro v-if="user.paidTier" class="account-area__wrap__left__tier-badge" />
                                <TierBadgeFree v-else class="account-area__wrap__left__tier-badge" />
                            </template>
                        </div>
                        <ArrowIcon class="account-area__wrap__arrow" />
                    </div>
                    <div v-if="isAccountDropdownShown" class="account-area__dropdown">
                        <div class="account-area__dropdown__header">
                            <div class="account-area__dropdown__header__left">
                                <SatelliteIcon />
                                <h2 class="account-area__dropdown__header__left__label">Satellite</h2>
                            </div>
                            <div class="account-area__dropdown__header__right">
                                <p class="account-area__dropdown__header__right__sat">{{ satellite }}</p>
                                <a
                                    class="account-area__dropdown__header__right__link"
                                    href="https://docs.storj.io/dcs/concepts/satellite"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    <InfoIcon />
                                </a>
                            </div>
                        </div>
                        <div v-if="!user.paidTier && billingEnabled" tabindex="0" class="account-area__dropdown__item" @click="onUpgrade" @keyup.enter="onUpgrade">
                            <UpgradeIcon />
                            <p class="account-area__dropdown__item__label">Upgrade</p>
                        </div>
                        <div v-if="billingEnabled" class="account-area__dropdown__item" @click="navigateToBilling">
                            <BillingIcon />
                            <p class="account-area__dropdown__item__label">Billing</p>
                        </div>
                        <div class="account-area__dropdown__item" @click="navigateToSettings">
                            <SettingsIcon />
                            <p class="account-area__dropdown__item__label">Account Settings</p>
                        </div>
                        <div class="account-area__dropdown__item" @click="onLogout">
                            <LogoutIcon />
                            <p class="account-area__dropdown__item__label">Logout</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/types/router';
import { NavigationLink } from '@/types/navigation';
import { Project } from '@/types/projects';
import { User } from '@/types/users';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { LocalData } from '@/utils/localData';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify } from '@/utils/hooks';
import { useABTestingStore } from '@/store/modules/abTestingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useCreateProjectClickHandler } from '@/composables/useCreateProjectClickHandler';

import ResourcesLinks from '@/components/navigation/ResourcesLinks.vue';
import QuickStartLinks from '@/components/navigation/QuickStartLinks.vue';
import VLoader from '@/components/common/VLoader.vue';

import CrossIcon from '@/../static/images/common/closeCross.svg';
import LogoIcon from '@/../static/images/logo.svg';
import AccessGrantsIcon from '@/../static/images/navigation/accessGrants.svg';
import AccountIcon from '@/../static/images/navigation/account.svg';
import ArrowIcon from '@/../static/images/navigation/arrowExpandRight.svg';
import BillingIcon from '@/../static/images/navigation/billing.svg';
import BucketsIcon from '@/../static/images/navigation/buckets.svg';
import UpgradeIcon from '@/../static/images/navigation/upgrade.svg';
import CheckmarkIcon from '@/../static/images/navigation/checkmark.svg';
import CreateProjectIcon from '@/../static/images/navigation/createProject.svg';
import InfoIcon from '@/../static/images/navigation/info.svg';
import LogoutIcon from '@/../static/images/navigation/logout.svg';
import PassphraseIcon from '@/../static/images/navigation/passphrase.svg';
import MenuIcon from '@/../static/images/navigation/menu.svg';
import ProjectIcon from '@/../static/images/navigation/project.svg';
import DashboardIcon from '@/../static/images/navigation/projectDashboard.svg';
import QuickStartIcon from '@/../static/images/navigation/quickStart.svg';
import ResourcesIcon from '@/../static/images/navigation/resources.svg';
import SatelliteIcon from '@/../static/images/navigation/satellite.svg';
import SettingsIcon from '@/../static/images/navigation/settings.svg';
import TierBadgeFree from '@/../static/images/navigation/tierBadgeFree.svg';
import TierBadgePro from '@/../static/images/navigation/tierBadgePro.svg';
import UsersIcon from '@/../static/images/navigation/users.svg';

const FIRST_PAGE = 1;
const navigation: NavigationLink[] = [
    RouteConfig.ProjectDashboard.withIcon(DashboardIcon),
    RouteConfig.Buckets.withIcon(BucketsIcon),
    RouteConfig.AccessGrants.withIcon(AccessGrantsIcon),
    RouteConfig.Team.withIcon(UsersIcon),
];

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const pmStore = useProjectMembersStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const abTestingStore = useABTestingStore();
const notificationsStore = useNotificationsStore();
const projectsStore = useProjectsStore();
const obStore = useObjectBrowserStore();

const router = useRouter();
const route = useRoute();
const notify = useNotify();
const { handleCreateProjectClick } = useCreateProjectClickHandler();

const auth: AuthHttpApi = new AuthHttpApi();

const isResourcesDropdownShown = ref<boolean>(false);
const isQuickStartDropdownShown = ref<boolean>(false);
const isProjectDropdownShown = ref<boolean>(false);
const isAccountDropdownShown = ref<boolean>(false);
const isOpened = ref<boolean>(false);
const isLoading = ref<boolean>(false);

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.state.config.billingFeaturesEnabled);

/**
 * Whether the user is the owner of the selected project.
 */
const isProjectOwner = computed((): boolean => {
    return usersStore.state.user.id === projectsStore.state.selectedProject.ownerId;
});

/**
 * Returns user's own projects.
 */
const ownProjects = computed((): Project[] => {
    const projects = projectsStore.projects.filter((p) => p.ownerId === usersStore.state.user.id);
    return projects.sort(compareProjects);
});

/**
 * Returns projects the user is invited to.
 */
const sharedProjects = computed((): Project[] => {
    const projects = projectsStore.projects.filter((p) => p.ownerId !== usersStore.state.user.id);
    return projects.sort(compareProjects);
});

/**
 * Indicates if current route is objects view.
 */
const isBucketsView = computed((): boolean => {
    return route.path.includes(RouteConfig.BucketsManagement.path);
});

/**
 * Returns selected project from store.
 */
const selectedProject = computed((): Project => {
    return projectsStore.state.selectedProject;
});

/**
 * Returns satellite name from store.
 */
const satellite = computed((): string => {
    return configStore.state.config.satelliteName;
});

/**
 * Returns user entity from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});

/**
 * This comparator is used to sort projects by isSelected.
 */
function compareProjects(a: Project, b: Project) {
    if (a.isSelected) return -1;
    if (b.isSelected) return 1;
    return 0;
}

/**
 * Redirects to project dashboard.
 */
function onLogoClick(): void {
    router.push(RouteConfig.AllProjectsDashboard.path);
}

function onNavClick(path: string): void {
    trackClickEvent(path);
    isOpened.value = false;
}

/**
 * Toggles navigation content visibility.
 */
function toggleNavigation(): void {
    isOpened.value = !isOpened.value;
}

/**
 * Toggles resources dropdown visibility.
 */
function toggleResourcesDropdown(): void {
    isResourcesDropdownShown.value = !isResourcesDropdownShown.value;
}

/**
 * Toggles quick start dropdown visibility.
 */
function toggleQuickStartDropdown(): void {
    isQuickStartDropdownShown.value = !isQuickStartDropdownShown.value;
}

/**
 * Toggles projects dropdown visibility.
 */
function toggleProjectDropdown(): void {
    isProjectDropdownShown.value = !isProjectDropdownShown.value;
}

/**
 * Toggles account dropdown visibility.
 */
function toggleAccountDropdown(): void {
    isAccountDropdownShown.value = !isAccountDropdownShown.value;
    window.scrollTo(0, document.querySelector('.navigation-area__container__wrap')?.scrollHeight || 0);
}

/**
 * Sends new path click event to segment.
 */
function trackClickEvent(path: string): void {
    analyticsStore.pageVisit(path);
}

/**
 * Route to all projects page.
 */
function onAllProjectsClick(): void {
    analyticsStore.pageVisit(RouteConfig.AllProjectsDashboard.path);
    router.push(RouteConfig.AllProjectsDashboard.path);
    toggleProjectDropdown();
}

/**
 * Route to project details page.
 */
function onProjectDetailsClick(): void {
    analyticsStore.pageVisit(RouteConfig.EditProjectDetails.path);
    router.push(RouteConfig.EditProjectDetails.path);
    toggleProjectDropdown();
}

/**
 * Toggles manage passphrase modal shown.
 */
function onManagePassphraseClick(): void {
    appStore.updateActiveModal(MODALS.manageProjectPassphrase);
}

async function onProjectClick(): Promise<void> {
    toggleProjectDropdown();

    if (isLoading.value || !isProjectDropdownShown.value) return;

    isLoading.value = true;

    try {
        await projectsStore.getProjects();
        await projectsStore.getProjectLimits(selectedProject.value.id);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.MOBILE_NAVIGATION);
    } finally {
        isLoading.value = false;
    }
}

/**
 * Fetches all project related information.
 * @param projectID
 */
async function onProjectSelected(projectID: string): Promise<void> {
    analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
    projectsStore.selectProject(projectID);
    LocalData.setSelectedProjectId(projectID);
    pmStore.setSearchQuery('');

    isProjectDropdownShown.value = false;

    if (isBucketsView.value) {
        bucketsStore.clear();
        analyticsStore.pageVisit(RouteConfig.Buckets.path);
        await router.push(RouteConfig.Buckets.path).catch(() => {return; });
    }

    try {
        await Promise.all([
            billingStore.getProjectUsageAndChargesCurrentRollup(),
            pmStore.getProjectMembers(FIRST_PAGE, projectID),
            agStore.getAccessGrants(FIRST_PAGE, projectID),
            bucketsStore.getBuckets(FIRST_PAGE, projectID),
            projectsStore.getProjectLimits(projectID),
        ]);
    } catch (error) {
        error.message = `Unable to select project. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.MOBILE_NAVIGATION);
    }
}

/**
 * Route to create project page.
 */
function onCreateLinkClick(): void {
    handleCreateProjectClick();
    isProjectDropdownShown.value = false;
}

/**
 * Starts upgrade account flow.
 */
function onUpgrade(): void {
    isOpened.value = false;

    if (!billingEnabled.value) return;

    appStore.updateActiveModal(MODALS.upgradeAccount);
}

/**
 * Navigates user to billing page.
 */
function navigateToBilling(): void {
    isOpened.value = false;
    if (route.path.includes(RouteConfig.Billing.path)) return;

    const link = RouteConfig.Account.with(RouteConfig.Billing.with(RouteConfig.BillingOverview));
    router.push(link.path);
    analyticsStore.pageVisit(link.path);
}

/**
 * Navigates user to account settings page.
 */
function navigateToSettings(): void {
    isOpened.value = false;
    analyticsStore.pageVisit(RouteConfig.Account.with(RouteConfig.Settings).path);
    router.push(RouteConfig.Account.with(RouteConfig.Settings).path).catch(() => {return;});
}

/**
 * Logouts user and navigates to login page.
 */
async function onLogout(): Promise<void> {
    analyticsStore.pageVisit(RouteConfig.Login.path);
    await router.push(RouteConfig.Login.path);

    await Promise.all([
        pmStore.clear(),
        projectsStore.clear(),
        usersStore.clear(),
        agStore.stopWorker(),
        agStore.clear(),
        notificationsStore.clear(),
        bucketsStore.clear(),
        appStore.clear(),
        billingStore.clear(),
        abTestingStore.reset(),
        obStore.clear(),
    ]);

    try {
        analyticsStore.eventTriggered(AnalyticsEvent.LOGOUT_CLICKED);
        await auth.logout();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.MOBILE_NAVIGATION);
    }
}
</script>

<style scoped lang="scss">
.navigation-svg-path {
    fill: rgb(53 64 73);
}

.container-wrapper {
    width: 100%;
}

.navigation-area {
    background-color: #fff;
    font-family: 'font_regular', sans-serif;
    box-shadow: 0 0 32px rgb(0 0 0 / 4%);

    &__container {
        position: relative;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: space-between;
        overflow-x: hidden;
        overflow-y: auto;
        width: 100%;
        height: 100%;

        &__header {
            display: flex;
            width: 100%;
            box-sizing: border-box;
            padding: 0 24px;
            justify-content: space-between;
            align-items: center;
            height: 4rem;

            &__logo {
                height: 49px;
                width: auto;
            }
        }

        &__wrap {
            position: fixed;
            top: 4rem;
            left: 0;
            display: flex;
            flex-direction: column;
            align-items: center;
            width: 100%;
            z-index: 9999;
            overflow-y: auto;
            overflow-x: hidden;
            background: white;
            height: calc(var(--vh, 100vh) - 4rem);

            &.with-padding {
                padding-bottom: 3rem;
                height: calc(var(--vh, 100vh) - 7rem);
            }

            &__small-logo {
                display: none;
            }

            &__edit {
                margin: 10px 0 16px;
                width: 100%;
            }

            &__item-container {
                padding: 14px 32px;
                width: 100%;
                display: flex;
                align-items: center;
                justify-content: space-between;
                border-left: 4px solid #fff;
                color: var(--c-grey-6);
                position: static;
                cursor: pointer;
                height: 48px;
                box-sizing: border-box;

                &__left {
                    display: flex;
                    align-items: center;

                    &__label {
                        font-size: 14px;
                        line-height: 20px;
                        margin-left: 24px;
                    }
                }
            }

            &__border {
                margin: 0 32px 16px;
                border: 0.5px solid var(--c-grey-2);
                width: calc(100% - 48px);
            }
        }
    }
}

.router-link-active,
.active {
    border-color: #000;
    color: var(--c-blue-6);
    font-family: 'font_bold', sans-serif;

    :deep(path) {
        fill: #000;
    }
}

:deep(.dropdown-item) {
    display: flex;
    align-items: center;
    font-family: 'font_regular', sans-serif;
    padding: 16px;
    cursor: pointer;
    border-bottom: 1px solid var(--c-grey-2);
    background: var(--c-grey-1);
}

:deep(.dropdown-item__icon) {
    max-width: 40px;
    min-width: 40px;
}

:deep(.dropdown-item__text) {
    margin-left: 10px;
}

:deep(.dropdown-item__text__title) {
    font-family: 'font_bold', sans-serif;
    font-size: 14px;
    line-height: 22px;
    color: var(--c-blue-6);
}

:deep(.dropdown-item__text__label) {
    font-size: 12px;
    line-height: 21px;
    color: var(--c-blue-6);
}

:deep(.dropdown-item:first-of-type) {
    border-radius: 8px 8px 0 0;
}

:deep(.dropdown-item:last-of-type) {
    border-radius: 0 0 8px 8px;
}

:deep(.project-selection__dropdown) {
    all: unset !important;
    position: relative !important;
    display: flex;
    flex-direction: column;
    font-family: 'font_regular', sans-serif;
    padding: 10px 16px;
    cursor: pointer;
    border-top: 1px solid var(--c-grey-2);
    border-bottom: 1px solid var(--c-grey-2);
}

.project-selection {
    font-family: 'font_regular', sans-serif;
    position: relative;
    width: 100%;

    &__selected {
        box-sizing: border-box;
        padding: 22px 32px;
        border-left: 4px solid #fff;
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: space-between;
        cursor: pointer;
        position: static;

        &__left {
            display: flex;
            align-items: center;
            max-width: calc(100% - 16px);

            &__name {
                max-width: calc(100% - 24px - 16px);
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
                margin-left: 24px;
                white-space: nowrap;
                overflow: hidden;
                text-overflow: ellipsis;
            }

            &__placeholder {
                display: none;
            }
        }
    }

    &__dropdown {
        width: 100%;
        background-color: #fff;

        &__loader-container {
            margin: 10px 0;
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 8px 8px 0 0;
        }

        &__section-head {
            display: flex;
            align-items: center;
            gap: 24px;
            height: 48px;
            box-sizing: border-box;
            padding: 8px 32px;

            &.shared {
                border-top: 1px solid var(--c-grey-2);
            }

            &__tag {
                border: 1px solid var(--c-purple-2);
                border-radius: 24px;
                padding: 2px 8px;
                text-align: center;
                font-size: 12px;
                font-weight: 600;
                line-height: 18px;
                color: var(--c-purple-4);
                background: var(--c-white);

                &.shared {
                    border: 1px solid var(--c-yellow-2);
                    color: var(--c-yellow-5);
                }
            }
        }

        &__items {
            overflow-y: auto;
            background-color: #fff;

            &__choice {
                display: flex;
                align-items: center;
                padding: 16px 32px;
                cursor: pointer;
                height: 32px;

                &__selected,
                &__unselected {
                    font-size: 14px;
                    line-height: 20px;
                    color: #1b2533;
                    white-space: nowrap;
                    overflow: hidden;
                    text-overflow: ellipsis;
                }

                &__selected {
                    font-family: 'font_bold', sans-serif;
                    margin-left: 24px;
                }

                &__unselected {
                    padding-left: 40px;
                }

                &__mark-container {
                    width: 16px;
                    height: 16px;

                    &__image {
                        object-fit: cover;
                    }
                }
            }
        }

        &__link-container {
            padding: 16px 32px;
            height: 48px;
            cursor: pointer;
            display: flex;
            align-items: center;
            box-sizing: border-box;
            border-bottom: 1px solid var(--c-grey-2);

            &__label {
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
                margin-left: 24px;
            }

            &:last-of-type {
                border-radius: 0 0 8px 8px;
            }
        }
    }
}

.account-area {
    width: 100%;

    &__wrap {
        box-sizing: border-box;
        padding: 16px 32px 16px 36px;
        height: 48px;
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: space-between;
        cursor: pointer;
        position: static;

        &__left {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__label,
            &__label-small {
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
                margin: 0 6px 0 24px;
            }

            &__label-small {
                display: none;
                margin: 0;
            }
        }
    }

    &__dropdown {
        position: relative;
        background: #fff;
        width: 100%;
        box-sizing: border-box;

        &__header {
            background: var(--c-grey-1);
            padding: 16px 32px 16px 36px;
            border: 1px solid var(--c-grey-2);
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__left,
            &__right {
                display: flex;
                align-items: center;

                &__label {
                    font-size: 14px;
                    line-height: 20px;
                    color: var(--c-grey-6);
                    margin-left: 16px;
                }

                &__sat {
                    font-size: 14px;
                    line-height: 20px;
                    color: var(--c-grey-6);
                    margin-right: 16px;
                }

                &__link {
                    max-height: 16px;
                }
            }
        }

        &__item {
            display: flex;
            align-items: center;
            padding: 16px 32px 16px 36px;
            background: var(--c-grey-1);

            &__label {
                margin-left: 16px;
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
            }

            &:last-of-type {
                border-radius: 0 0 8px 8px;
            }
        }
    }
}
</style>
