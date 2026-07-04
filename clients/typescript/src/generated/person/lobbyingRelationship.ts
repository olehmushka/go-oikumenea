/**
 * A public lobbying filing (D-InstitutionalTies, M33) — a reified link: the person as registrant
 * lobbying for a client before a legislative body on a set of issues. pii:basic.
 *
 */
export interface ILobbyingRelationship {
    'id': string;
    'personId': string;
    'registrant': string;
    'client'?: string | null;
    'legislativeBody'?: string | null;
    'issues': Array<string>;
    'filingId'?: string | null;
    'sourceUrl'?: string | null;
    'validFrom'?: string | null;
    'validTo'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
