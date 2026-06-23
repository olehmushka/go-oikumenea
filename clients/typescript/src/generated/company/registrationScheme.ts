/** A registration-identifier scheme (LEI/DUNS/EDRPOU/VAT/EIN); isGlobal marks the worldwide spine. */
export interface IRegistrationScheme {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    'validatorPattern'?: string | null;
    'isGlobal': boolean;
    'status': string;
    'sortOrder'?: number | null;
}
