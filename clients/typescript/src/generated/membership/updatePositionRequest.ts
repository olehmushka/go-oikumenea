/** Update a position's title / required rank / sort order. `code` and unit are immutable. Omitted fields are unchanged. */
export interface IUpdatePositionRequest {
    'title'?: string | null;
    /** Set the advisory required rank; omitted leaves it unchanged (clearing it is an open seam). */
    'requiredRankId'?: string | null;
    'sortOrder'?: number | null;
}
