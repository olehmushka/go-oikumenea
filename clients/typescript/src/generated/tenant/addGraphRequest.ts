/** Add a graph to the registry (instance-admin; graph.manage). */
export interface IAddGraphRequest {
    'code': string;
    'name': string;
    /** Defaults to true. */
    'isAuthorityBearing'?: boolean | null;
}
