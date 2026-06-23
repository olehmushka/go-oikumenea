/** Update name/kind/country/dates/state. code is immutable; omitted fields are unchanged. */
export interface IUpdateInstitutionRequest {
    'name'?: string | null;
    'kindId'?: string | null;
    'countryId'?: string | null;
    'foundedOn'?: string | null;
    'closedOn'?: string | null;
    'state'?: string | null;
}
