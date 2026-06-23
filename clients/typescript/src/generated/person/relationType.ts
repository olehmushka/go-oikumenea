/** An instance-admin catalog entry for an open-ended person↔person relation label (D-PersonRelationships). Stable code + translatable name + category. */
export interface IRelationType {
    /** Stable, locale-agnostic identifier (D-Code); immutable by convention. */
    'code': string;
    /** The translatable label as a locale -> text map (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
    /** One of sponsorship | association | next_of_kin. */
    'category': string;
    /** One of active | retired. */
    'status': string;
    'sortOrder'?: number | null;
}
