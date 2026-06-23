/** A company's address, located via the shared M19 location; role distinguishes registered/operating/branch. */
export interface ICompanyLocation {
    'id': string;
    'companyId': string;
    'locationId': string;
    /** One of registered | operating | branch. */
    'role': string;
    'createdAt': string;
    'updatedAt': string;
}
