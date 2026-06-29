/** Add an org-kind domain to the catalog (instance-admin; domain.manage). */
export interface ICreateDomainRequest {
    'code': string;
    'name': string;
    'sortOrder'?: number | null;
}
