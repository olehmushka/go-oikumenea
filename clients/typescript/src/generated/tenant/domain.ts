/**
 * An org-kind catalog row (D-TenantOrganizations, M40): military/government/company/
 * university/church/public-org, … Classifies organizations and units; a directory attribute,
 * never a PDP input. `code` is the stable external reference; `name` is the locale->text map.
 *
 */
export interface IDomain {
    /** The domain's URN RID (carried as a plain string). */
    'id': string;
    /** Stable, locale-agnostic identifier (e.g. military, university). Immutable by convention. */
    'code': string;
    /** locale->text display name (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
    /** active | retired (catalog soft-state). */
    'status': string;
    'sortOrder'?: number | null;
}
