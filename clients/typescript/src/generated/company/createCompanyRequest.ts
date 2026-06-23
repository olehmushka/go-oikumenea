export interface ICreateCompanyRequest {
    'code': string;
    'legalName': string;
    'shortName'?: string | null;
    'legalFormId': string;
    'ownershipCategory'?: string | null;
    'countryId'?: string | null;
    'foundedOn'?: string | null;
}
