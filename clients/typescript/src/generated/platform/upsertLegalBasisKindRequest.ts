/** Add or update a lawful-basis catalog entry (instance-admin). */
export interface IUpsertLegalBasisKindRequest {
    'code': string;
    'name': string;
    /** art6 | art9. */
    'article': string;
    'sortOrder'?: number | null;
}
