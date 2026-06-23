export interface IUpsertCurriculumItemRequest {
    'courseId': string;
    'isRequired'?: boolean | null;
    'yearOfStudy'?: number | null;
    'creditAllocation'?: number | null;
    'semesterSlot'?: number | null;
}
