/** A research centre / institute / lab of an institution. */
export interface IResearchCentre {
    'id': string;
    'institutionId': string;
    'code': string;
    'name': string;
    'kind': string;
    'fundingSource'?: string | null;
    'foundedOn'?: string | null;
    'dissolvedOn'?: string | null;
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
