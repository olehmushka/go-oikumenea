/** Add a national-identifier scheme to the catalog. */
export interface ICreatePersonalCodeSchemeRequest {
    'code': string;
    'countryIso'?: string | null;
    'genericCategory': string;
    /** Default-locale label; translatable via the localization store. */
    'name': string;
    'validationRegex'?: string | null;
    'sortOrder'?: number | null;
}
