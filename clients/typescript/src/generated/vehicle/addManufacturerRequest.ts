/** Record that a company manufactures this brand (temporal). */
export interface IAddManufacturerRequest {
    'companyId': string;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
}
