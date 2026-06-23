/** A versioned snapshot of a program's requirements. */
export interface ICurriculumVersion {
    'id': string;
    'programId': string;
    'versionCode': string;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
