/** An instance-admin catalog entry naming a country-namespaced national-identifier scheme (ua-rnokpp, us-ssn). */
export interface IPersonalCodeScheme {
    /** The scheme id (D-Code); the natural key. Immutable by convention. */
    'code': string;
    /** Country RID of the scheme's country (resolve via GET /geo/countries; the field name is retained for compatibility). */
    'countryIso'?: string | null;
    /** Semantic grouping — one of tax-id | national-id | social-insurance | health-insurance | residence-permit | other. */
    'genericCategory': string;
    /** The translatable label as a locale -> text map (D-i18n). */
    'name': { [key: string]: string };
    /** Optional data-side fallback format check (a compiled validator takes precedence). */
    'validationRegex'?: string | null;
    /** One of active | retired. */
    'status': string;
    'sortOrder'?: number | null;
    'createdAt': string;
    'updatedAt': string;
}
