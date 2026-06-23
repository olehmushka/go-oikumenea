/** Fill a vacant position with a person (a membership referencing the position; its unit is the position's unit). */
export interface IFillPositionRequest {
    'personId': string;
    'orderItemId'?: string | null;
    'effectiveFrom'?: string | null;
}
