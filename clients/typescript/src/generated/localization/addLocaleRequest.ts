/** Add a supported locale (instance-admin; `locale.manage`). */
export interface IAddLocaleRequest {
    /** ISO 639-3 code; must be unique. */
    'code': string;
    'name': string;
    /** Defaults to true. */
    'enabled'?: boolean | null;
    'sortOrder'?: number | null;
}
