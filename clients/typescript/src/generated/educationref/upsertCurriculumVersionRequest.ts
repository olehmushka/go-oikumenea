export interface IUpsertCurriculumVersionRequest {
    'versionCode': string;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'status'?: string | null;
}
