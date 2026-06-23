/** A full transliterated name form for one locale (e.g. ukr native, eng Latin). Person-managed, NOT the localization store. */
export interface INameVariant {
    'id': string;
    'personId': string;
    /** The locale/script this form is for (an i18n_locales code). */
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
    'isPrimary': boolean;
}
