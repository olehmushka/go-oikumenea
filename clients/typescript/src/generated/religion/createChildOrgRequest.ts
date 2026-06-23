/**
 * Create a child religious-body unit under a parent in the canonical graph (a tenant unit + the
 * parent→child canonical edge + the child's profile + an optional primary classification).
 * Rejected with Religion:ChildCreationExcluded if the parent carries an excludes_child_creation policy.
 *
 */
export interface ICreateChildOrgRequest {
    'code': string;
    'name': string;
    'orgKindId'?: string | null;
    'primaryTaxonId'?: string | null;
    /** public | shadow (default public). */
    'visibility'?: string | null;
}
