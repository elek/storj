// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/utils/httpClient';
import { APIError } from '@/utils/error';
import { type NotificationSettings } from '@/extensions/notifications/types';

export class NotificationsHttpAPI {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/notifications';

    /**
     * Returns where the notifications of the logged in user are sent.
     *
     * @throws APIError
     */
    public async get(): Promise<NotificationSettings> {
        const result = await this.read(await this.client.get(this.ROOT_PATH), 'Cannot retrieve notification settings', 'telegram');

        return {
            telegram: {
                enabled: !!result.telegram?.enabled,
                botName: result.telegram?.botName ?? '',
                connected: !!result.telegram?.connected,
            },
        };
    }

    /**
     * Returns a link to the Telegram bot, which connects the chat it is opened in.
     *
     * @throws APIError
     */
    public async createTelegramLink(): Promise<string> {
        const result = await this.read(await this.client.post(`${this.ROOT_PATH}/telegram/link`, null), 'Cannot create Telegram link', 'url');
        return result.url;
    }

    /**
     * Sends a test message to the connected Telegram chat.
     *
     * @throws APIError
     */
    public async testTelegram(): Promise<void> {
        await this.read(await this.client.post(`${this.ROOT_PATH}/telegram/test`, null), 'Cannot send test message', 'sent');
    }

    /**
     * Disconnects the Telegram chat.
     *
     * @throws APIError
     */
    public async disconnectTelegram(): Promise<void> {
        await this.read(await this.client.delete(`${this.ROOT_PATH}/telegram`), 'Cannot disconnect Telegram', 'disconnected');
    }

    /**
     * N.B. an unrouted /api request falls through to the console's catch-all,
     * which answers 200 with index.html. The body is required to parse, and to
     * carry the expected field, so that such a response cannot pass for success.
     */
    private async read(response: Response, fallback: string, expectedField: string) {
        const result = await response.json().catch(() => null);

        if (!response.ok || !result?.[expectedField]) {
            throw new APIError({
                status: response.status,
                message: result?.error || fallback,
                requestID: response.headers.get('x-request-id'),
            });
        }

        return result;
    }
}
