/** Add a lobbying relationship, or replace one when id is supplied (D-InstitutionalTies, M33). */
export interface IUpsertLobbyingRelationshipRequest {
    'id'?: string | null;
    'registrant': string;
    'client'?: string | null;
    'legislativeBody'?: string | null;
    'issues'?: Array<string> | null;
    'filingId'?: string | null;
    'sourceUrl'?: string | null;
    'validFrom'?: string | null;
    'validTo'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
