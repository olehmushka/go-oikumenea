/** Edit a scheme. `code` is immutable by convention. */
export interface IUpdatePersonalCodeSchemeRequest {
    'countryIso'?: string | null;
    'genericCategory'?: string | null;
    'name'?: string | null;
    'validationRegex'?: string | null;
    'status'?: string | null;
    'sortOrder'?: number | null;
}
