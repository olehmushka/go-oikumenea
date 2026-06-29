/**
 * Create a top-level religious body (M41 / D-UnifiedOrgGraph): a `church`-domain
 * tenant organization + its root religious-body unit + the unit's profile + an optional primary
 * classification. The body has no parent (no canonical edge); descendants are added via
 * createChildOrg. `code`/`name` apply to both the organization and its root unit.
 *
 */
export interface ICreateRootOrgRequest {
    'code': string;
    'name': string;
    'orgKindId'?: string | null;
    'primaryTaxonId'?: string | null;
    /** public | shadow (default public). */
    'visibility'?: string | null;
}
