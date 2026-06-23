export interface ICreateBuildingRequest {
    'code': string;
    'name': string;
    'kind': string;
    'unitId'?: string | null;
    'locationId'?: string | null;
}
