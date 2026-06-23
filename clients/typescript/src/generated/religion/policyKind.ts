/** The data-driven org-policy vocabulary (e.g. excludes_child_creation). */
export interface IPolicyKind {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    'description'?: string | null;
    'status': string;
    'sortOrder'?: number | null;
}
