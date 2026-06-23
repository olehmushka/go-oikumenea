/** Move a unit under a new parent (same institution), recomputing the closure. Cycle-guarded. */
export interface IReparentUnitRequest {
    /** The new parent unit; null makes the unit top-level. */
    'parentId'?: string | null;
}
