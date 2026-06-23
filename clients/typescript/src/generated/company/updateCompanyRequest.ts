/** Update name/form/category/country/dates/state. code is immutable; omitted fields are unchanged. */
export interface IUpdateCompanyRequest {
    'legalName'?: string | null;
    'shortName'?: string | null;
    'legalFormId'?: string | null;
    'ownershipCategory'?: string | null;
    'countryId'?: string | null;
    'foundedOn'?: string | null;
    'dissolvedOn'?: string | null;
    'state'?: string | null;
}
