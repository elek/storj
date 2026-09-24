// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { type RouteRecordRaw } from 'vue-router';
import { Bell } from '@lucide/vue';

import { type ExtensionNavItem } from '@/extensions/types';

export const NOTIFICATIONS_PATH = '/notifications';

export const notificationsRoutes: RouteRecordRaw[] = [
    {
        path: NOTIFICATIONS_PATH,
        name: 'Notifications',
        component: () => import(/* webpackChunkName: "Notifications" */ '@/extensions/notifications/NotificationsView.vue'),
    },
];

export const notificationsNavItems: ExtensionNavItem[] = [
    {
        title: 'Notifications',
        to: NOTIFICATIONS_PATH,
        icon: Bell,
        // right below Nodes
        order: 110,
    },
];
