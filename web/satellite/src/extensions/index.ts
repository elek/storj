// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * This is the registry of satellite-specific console features.
 *
 * Each extension lives in its own subdirectory and exports its routes and its
 * navigation entries. Adding one means adding a subdirectory and two lines
 * here - the router and the navigation drawers pick it up on their own.
 */

import { type RouteRecordRaw } from 'vue-router';

import { type ExtensionNavItem } from '@/extensions/types';

export * from '@/extensions/types';

/**
 * Routes contributed by extensions. They are rendered inside the account
 * layout, so they are reachable from anywhere in the app.
 */
export const extensionRoutes: RouteRecordRaw[] = [];

/**
 * Navigation entries contributed by extensions, in display order.
 */
export const extensionNavItems: ExtensionNavItem[] = [];
