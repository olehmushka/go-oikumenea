/** Edit/reorder a category. Omitted fields are unchanged. `code` is immutable by convention. */
export interface IUpdateCategoryRequest {
    'name'?: string | null;
    'sortOrder'?: number | null;
}
