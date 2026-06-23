export interface IUpsertPublicationRequest {
    'code': string;
    'title': string;
    'institutionId'?: string | null;
    'kind'?: string | null;
    'doi'?: string | null;
    'venue'?: string | null;
    'publishedOn'?: string | null;
    'openAccess'?: boolean | null;
}
