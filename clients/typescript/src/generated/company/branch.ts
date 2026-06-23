/** A non-independent sub-unit BRANCH_OF a parent company (link__branch_of). */
export interface IBranch {
    'id': string;
    'branchId': string;
    'branchLabel'?: string | null;
    'parentId': string;
    'parentLabel'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
