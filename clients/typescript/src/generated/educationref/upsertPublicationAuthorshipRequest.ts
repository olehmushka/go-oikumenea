export interface IUpsertPublicationAuthorshipRequest {
    'publicationId': string;
    'authorOrder'?: number | null;
    'corresponding'?: boolean | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
}
