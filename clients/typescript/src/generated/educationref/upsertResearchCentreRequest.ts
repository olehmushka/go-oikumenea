export interface IUpsertResearchCentreRequest {
    'code': string;
    'name': string;
    'kind'?: string | null;
    'fundingSource'?: string | null;
    'foundedOn'?: string | null;
    'dissolvedOn'?: string | null;
    'status'?: string | null;
}
