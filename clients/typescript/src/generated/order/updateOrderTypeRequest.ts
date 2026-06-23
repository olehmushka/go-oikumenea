/** Edit an order type. `code`, `category`, and `effect` are immutable by convention. */
export interface IUpdateOrderTypeRequest {
    'name'?: string | null;
    'status'?: string | null;
    'sortOrder'?: number | null;
}
