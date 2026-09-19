// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { type Component } from 'vue';

/**
 * ExtensionNavItem is a single entry an extension contributes to the navigation
 * drawers. It is rendered in both the account and the project navigation, so
 * the item stays reachable wherever the user is.
 */
export interface ExtensionNavItem {
    title: string;
    /**
     * to is an absolute route path.
     */
    to: string;
    icon: Component;
    /**
     * order sorts the items among themselves; lower comes first.
     */
    order?: number;
}
