// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { Duration } from '@/utils/time';

/**
 * Exposes all user-related functionality.
 */
export interface UsersApi {
    /**
     * Updates users full name and short name.
     *
     * @param user - contains information that should be updated
     * @throws Error
     */
    update(user: UpdatedUser): Promise<void>;
    /**
     * Fetch user.
     *
     * @returns User
     * @throws Error
     */
    get(): Promise<User>;
    /**
     * Fetches user frozen status.
     *
     * @returns boolean
     * @throws Error
     */
    getFrozenStatus(): Promise<FreezeStatus>;

    /**
     * Fetches user frozen status.
     *
     * @returns UserSettings
     * @throws Error
     */
    getUserSettings(): Promise<UserSettings>;

    /**
     * Changes user's settings.
     *
     * @param data
     * @returns UserSettings
     * @throws Error
     */
    updateSettings(data: SetUserSettingsData): Promise<UserSettings>;

    /**
     * Enable user's MFA.
     *
     * @throws Error
     */
    enableUserMFA(passcode: string): Promise<void>;
    /**
     * Disable user's MFA.
     *
     * @throws Error
     */
    disableUserMFA(passcode: string, recoveryCode: string): Promise<void>;
    /**
     * Generate user's MFA secret.
     *
     * @throws Error
     */
    generateUserMFASecret(): Promise<string>;
    /**
     * Generate user's MFA recovery codes.
     *
     * @throws Error
     */
    generateUserMFARecoveryCodes(): Promise<string[]>;
    /**
     * Generate user's MFA recovery codes requiring a code.
     *
     * @throws Error
     */
    regenerateUserMFARecoveryCodes(passcode?: string, recoveryCode?: string): Promise<string[]>;
    /**
     * Request increase for user's project limit.
     *
     * @throws Error
     */
    requestProjectLimitIncrease(limit: string): Promise<void>;
}

/**
 * User class holds info for User entity.
 */
export class User {
    public constructor(
        public id: string = '',
        public fullName: string = '',
        public shortName: string = '',
        public email: string = '',
        public partner: string = '',
        public password: string = '',
        public projectLimit: number = 0,
        public projectStorageLimit: number = 0,
        public projectBandwidthLimit: number = 0,
        public projectSegmentLimit: number = 0,
        public paidTier: boolean = false,
        public isMFAEnabled: boolean = false,
        public isProfessional: boolean = false,
        public position: string = '',
        public companyName: string = '',
        public employeeCount: string = '',
        public haveSalesContact: boolean = false,
        public mfaRecoveryCodeCount: number = 0,
        public _createdAt: string | null = null,
        public signupPromoCode: string = '',
        public freezeStatus: FreezeStatus = new FreezeStatus(),
    ) { }

    public get createdAt(): Date | null {
        if (!this._createdAt) {
            return null;
        }
        const date = new Date(this._createdAt);
        if (date.toString().includes('Invalid')) {
            return null;
        }
        return date;
    }

    public getFullName(): string {
        return !this.shortName ? this.fullName : this.shortName;
    }
}

/**
 * User class holds info for updating User.
 */
export class UpdatedUser {
    public constructor(
        public fullName: string = '',
        public shortName: string = '',
    ) { }

    public setFullName(value: string): void {
        this.fullName = value.trim();
    }

    public setShortName(value: string): void {
        this.shortName = value.trim();
    }

    public isValid(): boolean {
        return !!this.fullName;
    }
}

/**
 * DisableMFARequest represents a request to disable multi-factor authentication.
 */
export class DisableMFARequest {
    public constructor(
        public passcode: string = '',
        public recoveryCode: string = '',
    ) { }
}

/**
 * TokenInfo represents an authentication token response.
 */
export class TokenInfo {
    public constructor(
        public token: string,
        public expiresAt: Date,
    ) { }
}

/**
 * UserSettings represents response from GET /auth/account/settings.
 */
export class UserSettings {
    public constructor(
        private _sessionDuration: number | null = null,
        public onboardingStart = false,
        public onboardingEnd = false,
        public passphrasePrompt = true,
        public onboardingStep: string | null = null,
    ) { }

    public get sessionDuration(): Duration | null {
        if (this._sessionDuration) {
            return new Duration(this._sessionDuration);
        }
        return null;
    }
}

export interface SetUserSettingsData {
    onboardingStart?: boolean
    onboardingEnd?: boolean;
    passphrasePrompt?: boolean;
    onboardingStep?: string | null;
    sessionDuration?: number;
}

/**
 * FreezeStatus represents a freeze-status endpoint response.
 */
export class FreezeStatus {
    public constructor(
        public frozen = false,
        public warned = false,
    ) { }
}
