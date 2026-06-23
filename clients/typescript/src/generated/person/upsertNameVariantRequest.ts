/** Add or replace the name variant for a locale (keyed by (person, locale)). */
export interface IUpsertNameVariantRequest {
    'locale': string;
    'displayName': string;
    'title'?: string | null;
    'given'?: string | null;
    'given2'?: string | null;
    'surname'?: string | null;
    'surnamePrefix'?: string | null;
    'surname2'?: string | null;
    'generation'?: string | null;
    'credentials'?: string | null;
    'preferred'?: string | null;
    'isPrimary'?: boolean | null;
}
