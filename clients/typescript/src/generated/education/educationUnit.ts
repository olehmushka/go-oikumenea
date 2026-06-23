/** A node in an institution's recursive structure tree (campus → faculty → department → chair). */
export interface IEducationUnit {
    'id': string;
    'institutionId': string;
    /** The parent unit RID; null for a top-level unit. */
    'parentId'?: string | null;
    'kindId': string;
    'code': string;
    'name': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
    /** Distance from the queried root (populated by list-by-institution via the closure). */
    'depth'?: number | null;
    'createdAt': string;
    'updatedAt': string;
}
