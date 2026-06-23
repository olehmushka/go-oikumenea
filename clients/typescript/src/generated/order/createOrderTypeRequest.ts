/** Add an order type to the catalog. */
export interface ICreateOrderTypeRequest {
    'code': string;
    /** Default-locale label; translatable via the localization store. */
    'name': string;
    /** One of personnel-list | appointment | leave-travel | discipline-incentive | duty-roster. */
    'category': string;
    /** One of membership-start | membership-end | rank-change | record-only. */
    'effect': string;
    'sortOrder'?: number | null;
}
