import { Visibility } from "./visibility";

/**
 * Create an organization (the realm). Seeds its `command` (default, locked authority-bearing,
 * undeletable) + `operational` graphs in the same transaction (D-TenantOrganizations, M40).
 *
 */
export interface ICreateOrganizationRequest {
    'code': string;
    'name': string;
    /** The organization's domain (its kind) RID. */
    'domainId': string;
    /** Defaults to PUBLIC. */
    'visibility'?: Visibility | null;
    'metadata'?: any | null;
}
