export interface ICreateUnitRequest {
    'code': string;
    'name': string;
    'kindId': string;
    /** The parent unit (same institution); omit for a top-level unit. */
    'parentId'?: string | null;
    'sortOrder'?: number | null;
}
