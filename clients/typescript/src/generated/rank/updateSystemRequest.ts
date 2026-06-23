/** Edit/reorder a system. Omitted fields are unchanged. `code` is immutable; `country` cannot be cleared (open seam). */
export interface IUpdateSystemRequest {
    'name'?: string | null;
    'country'?: string | null;
    'sortOrder'?: number | null;
}
