export interface IUpsertAccreditationEventRequest {
    'entityKind': string;
    'institutionId'?: string | null;
    'programId'?: string | null;
    'body'?: string | null;
    'bodyCountryId'?: string | null;
    'outcome'?: string | null;
    'reviewOn'?: string | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'notes'?: string | null;
}
