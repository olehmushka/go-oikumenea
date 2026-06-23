import { Visibility } from "./visibility";

/** Create a unit. `name` is the default-locale text; other locales are managed via LocalizationService. */
export interface ICreateUnitRequest {
    /** Optional human-readable code (omit for a non-separate sub-unit; D-UnitCodeLifecycle, M28). */
    'code'?: string | null;
    'name': string;
    'unitKind'?: string | null;
    'level'?: number | null;
    /** Defaults to PUBLIC. */
    'visibility'?: Visibility | null;
    'metadata'?: any | null;
}
