/** Create/update (by code) a vehicle-type catalog node. name is the default-locale fallback. */
export interface IUpsertVehicleTypeRequest {
    'code': string;
    'name': string;
    'parentId'?: string | null;
    'sortOrder'?: number | null;
}
