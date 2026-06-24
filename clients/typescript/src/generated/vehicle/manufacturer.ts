/** A brand is MANUFACTURED_BY a company (link__manufactured_by); temporal (changes with acquisitions). */
export interface IManufacturer {
    'id': string;
    'brandId': string;
    'companyId': string;
    /** Best-effort company legal name. */
    'companyLabel'?: string | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
