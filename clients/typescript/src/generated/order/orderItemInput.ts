/** One item in a create/edit request. Which targets are required is validated against the type's effect. */
export interface IOrderItemInput {
    'typeId': string;
    'personId': string;
    'unitId'?: string | null;
    'positionId'?: string | null;
    'rankId'?: string | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'note'?: string | null;
}
