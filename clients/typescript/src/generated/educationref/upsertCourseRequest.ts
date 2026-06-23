export interface IUpsertCourseRequest {
    'code': string;
    'title': string;
    'owningUnitId'?: string | null;
    'creditHours'?: number | null;
    'level'?: number | null;
    'description'?: string | null;
    'deliveryMode'?: string | null;
    'status'?: string | null;
}
