// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * TelegramSettings describes the Telegram chat of the logged in user.
 */
export interface TelegramSettings {
    /**
     * enabled is false when the satellite has no Telegram bot configured.
     */
    enabled: boolean;
    botName: string;
    /**
     * connected is true when the user has connected a chat.
     */
    connected: boolean;
}

/**
 * NotificationSettings is the response of the notifications endpoint.
 */
export interface NotificationSettings {
    telegram: TelegramSettings;
}
