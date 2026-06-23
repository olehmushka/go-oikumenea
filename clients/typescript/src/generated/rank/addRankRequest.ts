/** Add a rank under a (leaf) type. */
export interface IAddRankRequest {
    /** The URN RID of the owning (active) type. */
    'typeId': string;
    'code': string;
    'name': string;
    'abbreviation'?: string | null;
    /** Optional standardized cross-system grade (a GET /rank-grades code); validated on write. */
    'gradeCode'?: string | null;
    /** Seniority ordinal; defaults to appended last within the type. */
    'sortOrder'?: number | null;
}
