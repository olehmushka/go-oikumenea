import { UnitState } from "./unitState";
import { Visibility } from "./visibility";

/** A node in the org graph. `code` is the stable external reference; `name` is the locale->text map. */
export interface IUnit {
    /** The unit's URN RID (carried as a plain string). */
    'id': string;
    /** The owning organization's RID (D-TenantOrganizations, M40). */
    'orgId': string;
    /** The unit's domain (kind class) RID; defaults to the org's domain, may differ (mixed trees). Never a PDP input. */
    'domainId': string;
    /** The unit's domain-scoped unit-kind RID (replaces the former free-text unitKind). Never branched on in code. */
    'kindId'?: string | null;
    /** Optional human-readable business ID (D-UnitCodeLifecycle, M28); absent = a non-separate sub-unit. The RID (id) is the stable external handle. Unique among active units that have a code; set/corrected/cleared via PUT /units/{id}/code. */
    'code'?: string | null;
    /** locale->text display name (all enabled locales; default-locale fallback + i18n store). */
    'name': { [key: string]: string };
    /** Optional ordinal for sort/filter; never a PDP or shadow-gate input. */
    'level'?: number | null;
    'visibility': Visibility;
    'state': UnitState;
    /** Free-form long-tail attributes (JSONB). */
    'metadata'?: any | null;
    'createdAt': string;
    'updatedAt': string;
}
