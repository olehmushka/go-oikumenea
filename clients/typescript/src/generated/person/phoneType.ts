/** An instance-admin catalog entry naming a contact-phone kind. Stable code + translatable name. */
export interface IPhoneType {
    /** Stable, locale-agnostic identifier (D-Code); immutable by convention. */
    'code': string;
    /** The translatable label as a locale -> text map (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
    /** One of active | retired. */
    'status': string;
    'sortOrder'?: number | null;
}
