/** Add a domain-scoped unit kind (instance-admin; unit-kind.manage). */
export interface ICreateUnitKindRequest {
    'domainId': string;
    'code': string;
    'name': string;
    'attrSchema'?: any | null;
    'sortOrder'?: number | null;
}
