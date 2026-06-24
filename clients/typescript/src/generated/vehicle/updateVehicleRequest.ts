/** Update type/model/vin/color/date/attributes/status; omitted fields are unchanged. */
export interface IUpdateVehicleRequest {
    'typeId'?: string | null;
    'modelId'?: string | null;
    'vin'?: string | null;
    'color'?: string | null;
    'manufactureDate'?: string | null;
    'attributes'?: string | null;
    'status'?: string | null;
}
