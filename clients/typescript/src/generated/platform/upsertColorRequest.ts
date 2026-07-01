/** Add or update a color (instance-admin; `color.manage`). Upserts on (domain, code). */
export interface IUpsertColorRequest {
    /** eye | hair | vehicle. */
    'domain': string;
    'code': string;
    /** The default-locale display name (other locales arrive via the localization store). */
    'name': string;
    'hex'?: string | null;
    'sortOrder'?: number | null;
}
