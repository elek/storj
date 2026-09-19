// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Node is a storage node operated by the logged in user.
 */
export interface Node {
    id: string;
    address: string;
    lastIpPort: string;
    wallet: string;
    walletFeatures: string[];
    pieceCount: number;
    freeDisk: number;
    online: boolean;
    lastContactSuccess: Date;
    lastContactFailure: Date;
    vettedAt: Date | null;
    disqualified: Date | null;
    disqualificationReason: string | null;
    exitFinishedAt: Date | null;
    countryCode: string;
    version: string;
    createdAt: Date;
    /**
     * confirmed is true when the satellite has recorded an owner tag naming the
     * logged in user for this node.
     */
    confirmed: boolean;
}

/**
 * NodesPage is the response of the nodes endpoint.
 */
export interface NodesPage {
    nodes: Node[];
    /**
     * truncated is true when the operator has more nodes than the server returns.
     */
    truncated: boolean;
}

/**
 * NodeStatus is the single status a node is displayed under. A node can be in
 * more than one underlying state at once, so these are ordered by how much the
 * operator needs to know about them.
 */
export enum NodeStatus {
    Disqualified = 'Disqualified',
    Exited = 'Exited',
    Offline = 'Offline',
    Online = 'Online',
}

export function nodeStatus(node: Node): NodeStatus {
    if (node.disqualified) return NodeStatus.Disqualified;
    if (node.exitFinishedAt) return NodeStatus.Exited;
    return node.online ? NodeStatus.Online : NodeStatus.Offline;
}

export function nodeStatusColor(status: NodeStatus): string {
    switch (status) {
    case NodeStatus.Disqualified: return 'error';
    case NodeStatus.Exited: return 'default';
    case NodeStatus.Offline: return 'warning';
    case NodeStatus.Online: return 'success';
    }
}
