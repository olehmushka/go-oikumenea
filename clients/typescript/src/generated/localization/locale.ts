/** A supported language for the deployment. `code` is the stable ISO 639-3 identifier; `name` is the endonym. */
export interface ILocale {
    /** ISO 639-3 code (e.g. ukr, eng); the stable, locale-agnostic identifier. */
    'code': string;
    /** Endonym/display name (e.g. "Українська", "English"). */
    'name': string;
    'enabled': boolean;
    /** Exactly one enabled locale is the default; its value is the fallback when a translation is absent. */
    'isDefault': boolean;
    'sortOrder': number;
}
