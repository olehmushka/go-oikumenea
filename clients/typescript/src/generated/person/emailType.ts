/** An instance-admin catalog entry naming a contact-email kind. Stable code + translatable name. */
export interface IEmailType {
    /** Stable, locale-agnostic identifier (D-Code); immutable by convention. */
    'code': string;
    /** The translatable label as a locale -> text map (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
    /** One of active | retired. */
    'status': string;
    'sortOrder'?: number | null;
}
