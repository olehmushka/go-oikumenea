/** Update name/kind/sort order/status. code and institution are immutable. */
export interface IUpdateUnitRequest {
    'name'?: string | null;
    'kindId'?: string | null;
    'status'?: string | null;
    'sortOrder'?: number | null;
}
