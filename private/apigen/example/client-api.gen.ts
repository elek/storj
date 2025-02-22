// AUTOGENERATED BY private/apigen
// DO NOT EDIT.

import { HttpClient } from '@/utils/httpClient';
import { Time, UUID } from '@/types/common';

export class Document {
    id: UUID;
    date: Time;
    pathParam: string;
    body: string;
    version: Version;
    metadata: Metadata;
}

export class Metadata {
    owner?: string;
    tags: string[][] | null;
}

export class NewDocument {
    content: string;
}

export class User {
    name: string;
    surname: string;
    email: string;
}

export class Version {
    date: Time;
    number: number;
}

class APIError extends Error {
    constructor(
        public readonly msg: string,
        public readonly responseStatusCode?: number,
    ) {
        super(msg);
    }
}

export class DocumentsHttpApiV0 {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/docs';

    public async get(): Promise<Document[]> {
        const fullPath = `${this.ROOT_PATH}/`;
        const response = await this.http.get(fullPath);
        if (response.ok) {
            return response.json().then((body) => body as Document[]);
        }
        const err = await response.json();
        throw new APIError(err.error, response.status);
    }

    public async getOne(path: string): Promise<Document> {
        const fullPath = `${this.ROOT_PATH}/${path}`;
        const response = await this.http.get(fullPath);
        if (response.ok) {
            return response.json().then((body) => body as Document);
        }
        const err = await response.json();
        throw new APIError(err.error, response.status);
    }

    public async getTag(path: string, tagName: string): Promise<string[]> {
        const fullPath = `${this.ROOT_PATH}/${path}/${tagName}`;
        const response = await this.http.get(fullPath);
        if (response.ok) {
            return response.json().then((body) => body as string[]);
        }
        const err = await response.json();
        throw new APIError(err.error, response.status);
    }

    public async getVersions(path: string): Promise<Version[]> {
        const fullPath = `${this.ROOT_PATH}/${path}`;
        const response = await this.http.get(fullPath);
        if (response.ok) {
            return response.json().then((body) => body as Version[]);
        }
        const err = await response.json();
        throw new APIError(err.error, response.status);
    }

    public async updateContent(request: NewDocument, path: string, id: UUID, date: Time): Promise<Document> {
        const u = new URL(`${this.ROOT_PATH}/${path}`, window.location.href);
        u.searchParams.set('id', id);
        u.searchParams.set('date', date);
        const fullPath = u.toString();
        const response = await this.http.post(fullPath, JSON.stringify(request));
        if (response.ok) {
            return response.json().then((body) => body as Document);
        }
        const err = await response.json();
        throw new APIError(err.error, response.status);
    }
}

export class UsersHttpApiV0 {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/users';

    public async get(): Promise<User[]> {
        const fullPath = `${this.ROOT_PATH}/`;
        const response = await this.http.get(fullPath);
        if (response.ok) {
            return response.json().then((body) => body as User[]);
        }
        const err = await response.json();
        throw new APIError(err.error, response.status);
    }

    public async create(request: User[]): Promise<void> {
        const fullPath = `${this.ROOT_PATH}/`;
        const response = await this.http.post(fullPath, JSON.stringify(request));
        if (response.ok) {
            return;
        }
        const err = await response.json();
        throw new APIError(err.error, response.status);
    }
}
