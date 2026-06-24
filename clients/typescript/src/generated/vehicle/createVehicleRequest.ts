export interface ICreateVehicleRequest {
    'typeId': string;
    'modelId'?: string | null;
    'vin'?: string | null;
    'color'?: string | null;
    'manufactureDate'?: string | null;
    /** A JSON object string; defaults to {}. */
    'attributes'?: string | null;
}
