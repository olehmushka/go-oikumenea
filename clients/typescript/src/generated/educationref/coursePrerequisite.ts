/** A course requires another course (required/recommended/corequisite). */
export interface ICoursePrerequisite {
    'id': string;
    'courseId': string;
    'requiredCourseId': string;
    'kind': string;
    'minGrade'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
