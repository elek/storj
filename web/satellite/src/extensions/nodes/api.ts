// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/utils/httpClient';
import { APIError } from '@/utils/error';
import { type Node, type NodesPage } from '@/extensions/nodes/types';

export class NodesHttpAPI {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/nodes';

    /**
     * Returns the storage nodes whose operator email matches the user's email.
     *
     * @throws APIError
     */
    public async get(): Promise<NodesPage> {
        const response = await this.client.get(this.ROOT_PATH);
        const result = await response.json();

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Cannot retrieve nodes',
                requestID: response.headers.get('x-request-id'),
            });
        }

        return {
            truncated: !!result.truncated,
            nodes: (result.nodes ?? []).map(parseNode),
        };
    }

    /**
     * Records that the logged in user owns the node, as a satellite signed tag.
     *
     * @throws APIError
     */
    public async confirm(nodeID: string): Promise<void> {
        const response = await this.client.post(`${this.ROOT_PATH}/${nodeID}/confirm`, null);

        if (!response.ok) {
            const result = await response.json().catch(() => ({}));
            throw new APIError({
                status: response.status,
                message: result.error || 'Cannot confirm node ownership',
                requestID: response.headers.get('x-request-id'),
            });
        }
    }
}

function parseNode(node: Node): Node {
    return {
        ...node,
        confirmed: !!node.confirmed,
        walletFeatures: node.walletFeatures ?? [],
        lastContactSuccess: new Date(node.lastContactSuccess),
        lastContactFailure: new Date(node.lastContactFailure),
        vettedAt: node.vettedAt ? new Date(node.vettedAt) : null,
        disqualified: node.disqualified ? new Date(node.disqualified) : null,
        exitFinishedAt: node.exitFinishedAt ? new Date(node.exitFinishedAt) : null,
        createdAt: new Date(node.createdAt),
    };
}
