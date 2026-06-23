/** A person STUDIED_AT an institution (optionally a unit/group), with an ISCED level + qualification. */
export interface IEnrollment {
    'id': string;
    'personId': string;
    'institutionId': string;
    'unitId'?: string | null;
    'groupId'?: string | null;
    /** The program studied (education_programs); null if unspecified. */
    'programId'?: string | null;
    'degreeLevelId'?: string | null;
    'fieldOfStudy'?: string | null;
    'studentNumber'?: string | null;
    /** One of enrolled | graduated | withdrawn | expelled | on_leave. */
    'status': string;
    'qualification'?: string | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
