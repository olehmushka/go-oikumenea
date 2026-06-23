/**
 * Add a type, rooted either directly under a category (categoryId) or nested under a parent
 * type (parentTypeId — it then inherits the parent's category). Supply exactly one; a parent
 * type that already holds ranks cannot gain child types (ranks live on leaf types only).
 *
 */
export interface IAddTypeRequest {
    /** The URN RID of the owning (active) category, for a root type. Omit when nesting under a parent type. */
    'categoryId'?: string | null;
    /** The URN RID of the owning (active) parent type, for a nested type. Omit for a root type. */
    'parentTypeId'?: string | null;
    'code': string;
    'name': string;
    /** Seniority ordinal; defaults to appended last among the new type's active siblings. */
    'sortOrder'?: number | null;
}
