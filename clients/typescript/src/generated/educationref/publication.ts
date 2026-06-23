/** An academic publication (optionally tied to an institution). */
export interface IPublication {
    'id': string;
    'institutionId'?: string | null;
    'code': string;
    'title': string;
    'kind': string;
    'doi'?: string | null;
    'venue'?: string | null;
    'publishedOn'?: string | null;
    'openAccess': boolean;
    'createdAt': string;
    'updatedAt': string;
}
