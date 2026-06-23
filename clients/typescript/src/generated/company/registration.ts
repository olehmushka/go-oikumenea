/** A company's per-scheme registration identifier (LEI / national number / VAT …). */
export interface IRegistration {
    'id': string;
    'companyId': string;
    'schemeId': string;
    'identifier': string;
    'validated': boolean;
    'createdAt': string;
    'updatedAt': string;
}
