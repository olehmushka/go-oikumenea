export interface ICreateInstitutionRequest {
    'code': string;
    'name': string;
    'kindId': string;
    'countryId'?: string | null;
    'foundedOn'?: string | null;
    'closedOn'?: string | null;
}
