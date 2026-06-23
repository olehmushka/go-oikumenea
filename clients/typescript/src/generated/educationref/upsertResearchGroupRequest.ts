export interface IUpsertResearchGroupRequest {
    'code': string;
    'name': string;
    'centreId'?: string | null;
    'unitId'?: string | null;
    'focusArea'?: string | null;
    'status'?: string | null;
}
