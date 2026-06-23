export interface ICreateCoursePrerequisiteRequest {
    'requiredCourseId': string;
    'kind'?: string | null;
    'minGrade'?: string | null;
}
