/** An instance-admin catalog entry naming a social network / messenger (D-PersonSocialChannels). Stable code + translatable name + category. */
export interface IPlatform {
    /** Stable, locale-agnostic identifier (D-Code); immutable by convention. */
    'code': string;
    /** The translatable label as a locale -> text map (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
    /** One of messenger | social. Only messenger platforms may carry a messenger link. */
    'category': string;
    /** One of active | retired. */
    'status': string;
    'sortOrder'?: number | null;
}
