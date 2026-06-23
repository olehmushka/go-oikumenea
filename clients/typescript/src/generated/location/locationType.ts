/** An instance-admin catalog label classifying a place (building/address/online); descriptive only. */
export interface ILocationType {
    'id': string;
    'code': string;
    /** Localized display name (locale->text), all enabled locales. */
    'name': { [key: string]: string };
    /** active | retired. */
    'status': string;
}
