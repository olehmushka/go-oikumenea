export interface ICreateVehicleRequest {
    'typeId': string;
    'modelId'?: string | null;
    'vin'?: string | null;
    /** A platform_colors RID (domain='vehicle', D-Color). */
    'colorId'?: string | null;
    'manufactureDate'?: string | null;
    /** A JSON object string; defaults to {}. */
    'attributes'?: string | null;
}
