/** Add a graph (instance-admin; graph.manage). Per-org when orgId is set, else instance-global (M40). */
export interface IAddGraphRequest {
    /** The organization the new graph belongs to; omit for an instance-global/cross-org graph. */
    'orgId'?: string | null;
    'code': string;
    'name': string;
    /** Defaults to true. */
    'isAuthorityBearing'?: boolean | null;
}
