/** A bank-account kind (current/savings/deposit/loan…); instance-extensible. */
export interface IAccountType {
    'id': string;
    'code': string;
    /** Default-locale fallback + i18n translations. */
    'name': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
}
