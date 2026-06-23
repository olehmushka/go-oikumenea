/** Edit/reorder a type. Omitted fields are unchanged. `code` is immutable. */
export interface IUpdateTypeRequest {
    'name'?: string | null;
    'sortOrder'?: number | null;
}
