/** A directed parent->child relationship within one graph (link__parent_of). */
export interface IUnitEdge {
    /** The edge's URN RID (carried as a plain string). */
    'id': string;
    /** The graph code this edge belongs to. */
    'graph': string;
    'parentId': string;
    'childId': string;
    'createdAt': string;
}
