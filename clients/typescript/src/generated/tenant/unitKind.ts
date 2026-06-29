/**
 * A domain-scoped unit-kind catalog row (D-TenantOrganizations, M40) replacing the former
 * free-text unitKind (military→brigade/battalion/platoon; university→faculty/department/chair).
 *
 */
export interface IUnitKind {
    /** The unit-kind's URN RID. */
    'id': string;
    /** The owning domain's RID; codes are unique per domain. */
    'domainId': string;
    /** Stable identifier, unique among active kinds of the same domain. */
    'code': string;
    /** locale->text display name. */
    'name': { [key: string]: string };
    /** Optional JSON schema validating a unit's metadata for this kind. */
    'attrSchema'?: any | null;
    /** active | retired. */
    'status': string;
    'sortOrder'?: number | null;
}
