export interface ILanguoidEntry {
    'rid': string;
    /** Glottocode. */
    'code': string;
    'name': string;
    /** The ISO 639-3 code, when the languoid has one. */
    'iso6393'?: string | null;
    'level'?: string | null;
    'familyCode'?: string | null;
}
