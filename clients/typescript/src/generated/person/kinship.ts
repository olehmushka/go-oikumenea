/** A directional parent→child blood/legal parentage link (D-PersonRelationships; Link link__kin_parent_of). Siblings are derived, never stored. */
export interface IKinship {
    'id': string;
    /** The parent's URN RID. */
    'parentId': string;
    /** The child's URN RID. */
    'childId': string;
    /** One of active | disestablished. */
    'status': string;
}
