/** A physical vehicle at registry grade. Soft-deleted, not destroyed. The RID is the external handle. */
export interface IVehicle {
    'id': string;
    'typeId': string;
    /** Best-effort default-locale type name. */
    'typeLabel'?: string | null;
    'modelId'?: string | null;
    'modelLabel'?: string | null;
    /** The brand of the model (derived), when a model is set. */
    'brandId'?: string | null;
    'brandLabel'?: string | null;
    'vin'?: string | null;
    'color'?: string | null;
    'manufactureDate'?: string | null;
    /** Long-tail spec grab-bag as a JSON string (DS-53 will column-ize stabilized fields). */
    'attributes'?: string | null;
    /** One of active | scrapped | exported. */
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
