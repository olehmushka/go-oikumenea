import { Visibility } from "./visibility";

/** Update name/kind/level/metadata/visibility. Omitted fields are unchanged. `code` is immutable. */
export interface IUpdateUnitRequest {
    'name'?: string | null;
    'unitKind'?: string | null;
    'level'?: number | null;
    'visibility'?: Visibility | null;
    'metadata'?: any | null;
}
