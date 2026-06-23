/** Edit/reorder a rank. Omitted fields are unchanged. `code` is immutable; `gradeCode` cannot be cleared (open seam). */
export interface IUpdateRankRequest {
    'name'?: string | null;
    'abbreviation'?: string | null;
    'gradeCode'?: string | null;
    'sortOrder'?: number | null;
}
