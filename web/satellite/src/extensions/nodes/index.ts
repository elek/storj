// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { type RouteRecordRaw } from 'vue-router';
import { HardDrive } from '@lucide/vue';

import { type ExtensionNavItem } from '@/extensions/types';

export const NODES_PATH = '/nodes';

export const nodesRoutes: RouteRecordRaw[] = [
    {
        path: NODES_PATH,
        name: 'Nodes',
        component: () => import(/* webpackChunkName: "Nodes" */ '@/extensions/nodes/NodesView.vue'),
    },
];

export const nodesNavItems: ExtensionNavItem[] = [
    {
        title: 'Nodes',
        to: NODES_PATH,
        icon: HardDrive,
        order: 100,
    },
];
