import { Visibility } from "./visibility";

/** Create a unit. `name` is the default-locale text; other locales are managed via LocalizationService. */
export interface ICreateUnitRequest {
    /** The owning organization's RID (required; D-TenantOrganizations, M40). */
    'orgId': string;
    /** The unit's domain RID; defaults to the organization's domain when omitted. May differ (mixed trees). */
    'domainId'?: string | null;
    /** The unit's domain-scoped unit-kind RID (replaces free-text unitKind). Must belong to the unit's domain. */
    'kindId'?: string | null;
    /** Optional human-readable code (omit for a non-separate sub-unit; D-UnitCodeLifecycle, M28). */
    'code'?: string | null;
    'name': string;
    'level'?: number | null;
    /** Defaults to PUBLIC. */
    'visibility'?: Visibility | null;
    'metadata'?: any | null;
}
