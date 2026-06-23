/** Attach the path unit as a child of `parentId` within a graph. */
export interface IAddEdgeRequest {
    'parentId': string;
    /** The graph code; defaults to command. */
    'graph'?: string | null;
}
