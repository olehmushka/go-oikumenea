/** An accreditation review cycle against an institution OR a program. */
export interface IAccreditationEvent {
    'id': string;
    'entityKind': string;
    'institutionId'?: string | null;
    'programId'?: string | null;
    'body'?: string | null;
    'bodyCountryId'?: string | null;
    'outcome': string;
    'reviewOn'?: string | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'notes'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
