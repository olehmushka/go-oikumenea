/** Move a taxon under a new parent, recomputing the closure. Cycle-guarded. */
export interface IReparentTaxonRequest {
    /** The new parent taxon; null makes the taxon a root religion. */
    'parentId'?: string | null;
}
