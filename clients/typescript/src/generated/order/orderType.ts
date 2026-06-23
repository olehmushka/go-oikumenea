/** An instance-admin catalog entry naming an order kind (arrival, appoint, leave-annual). Stable code + translatable name, with a category and an effect. */
export interface IOrderType {
    /** The order type's URN RID (carried as a plain string). */
    'id': string;
    /** Stable, locale-agnostic identifier (D-Code); immutable by convention. */
    'code': string;
    /** The translatable label as a locale -> text map (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
    /** One of personnel-list | appointment | leave-travel | discipline-incentive | duty-roster. */
    'category': string;
    /** The downstream consequence — one of membership-start | membership-end | rank-change | record-only. */
    'effect': string;
    /** One of active | retired. */
    'status': string;
    'sortOrder'?: number | null;
    'createdAt': string;
    'updatedAt': string;
}
