/** A course placed in a curriculum version (required/elective + credit/year metadata). */
export interface ICurriculumItem {
    'id': string;
    'versionId': string;
    'courseId': string;
    'isRequired': boolean;
    'yearOfStudy'?: number | null;
    'creditAllocation'?: number | null;
    'semesterSlot'?: number | null;
    'createdAt': string;
    'updatedAt': string;
}
