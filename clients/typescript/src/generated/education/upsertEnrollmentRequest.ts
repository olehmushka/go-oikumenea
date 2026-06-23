/** Create or update a person's enrollment. On create, omit nothing required; on update, target by id. */
export interface IUpsertEnrollmentRequest {
    'institutionId': string;
    'unitId'?: string | null;
    'groupId'?: string | null;
    'programId'?: string | null;
    'degreeLevelId'?: string | null;
    'fieldOfStudy'?: string | null;
    'studentNumber'?: string | null;
    'status'?: string | null;
    'qualification'?: string | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
}
