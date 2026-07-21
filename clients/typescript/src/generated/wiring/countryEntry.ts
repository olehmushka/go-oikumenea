export interface ICountryEntry {
    'rid': string;
    /** ISO-3166-1 alpha-2. */
    'code': string;
    /** Default-locale fallback + i18n translations. */
    'name': { [key: string]: string };
}
