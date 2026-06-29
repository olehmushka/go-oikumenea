import { UnitState } from "./unitState";
import { Visibility } from "./visibility";

/**
 * The realm a person joins (D-TenantOrganizations, M40): US Army / Bundeswehr / KhNU. Many
 * organizations may share a domain. Owns units and per-org graphs. A directory attribute,
 * never a PDP input — authority flows only through its graphs.
 *
 */
export interface IOrganization {
    /** The organization's URN RID. */
    'id': string;
    /** Stable, locale-agnostic identifier (D-Code; NOT NULL UNIQUE, immutable by convention). */
    'code': string;
    /** locale->text display name. */
    'name': { [key: string]: string };
    /** The organization's domain (its kind) RID. */
    'domainId': string;
    'visibility': Visibility;
    'state': UnitState;
    /** Free-form long-tail attributes (JSONB). */
    'metadata'?: any | null;
    'createdAt': string;
    'updatedAt': string;
}
