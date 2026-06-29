import { Visibility } from "./visibility";

/** Update name/kind/domain/level/metadata/visibility. Omitted fields are unchanged. `code` is excluded — set it via PUT /units/{id}/code (D-UnitCodeLifecycle). `orgId` is immutable. */
export interface IUpdateUnitRequest {
    'name'?: string | null;
    /** Re-classify the unit's domain (mixed trees allowed). The kind, if set, must match. */
    'domainId'?: string | null;
    'kindId'?: string | null;
    'level'?: number | null;
    'visibility'?: Visibility | null;
    'metadata'?: any | null;
}
