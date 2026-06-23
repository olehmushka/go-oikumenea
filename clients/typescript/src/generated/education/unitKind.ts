/** An instance-admin catalog entry classifying a structure-tree node (faculty/department/…). */
export interface IUnitKind {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
}
