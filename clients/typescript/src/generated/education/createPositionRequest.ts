/** Create a billet (vacant) owned by the institution or one of its units. */
export interface ICreatePositionRequest {
    'code': string;
    'title': string;
    'unitId'?: string | null;
    'sortOrder'?: number | null;
}
