/** Add a government position, or replace one when id is supplied (D-InstitutionalTies, M33). */
export interface IUpsertGovernmentPositionRequest {
    'id'?: string | null;
    'title': string;
    'body': string;
    'orgId'?: string | null;
    'countryId'?: string | null;
    'level'?: string | null;
    'roleType'?: string | null;
    'validFrom'?: string | null;
    'validTo'?: string | null;
    'pepTrigger'?: boolean | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
