/** Update type/model/vin/colorId/date/attributes/status; omitted fields are unchanged. */
export interface IUpdateVehicleRequest {
    'typeId'?: string | null;
    'modelId'?: string | null;
    'vin'?: string | null;
    /** A platform_colors RID (domain='vehicle', D-Color). */
    'colorId'?: string | null;
    'manufactureDate'?: string | null;
    'attributes'?: string | null;
    'status'?: string | null;
}
