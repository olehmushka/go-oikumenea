import { UnitState } from "./unitState";
import { Visibility } from "./visibility";

/** A node in the org graph. `code` is the stable external reference; `name` is the locale->text map. */
export interface IUnit {
    /** The unit's URN RID (carried as a plain string). */
    'id': string;
    /** Optional human-readable business ID (D-UnitCodeLifecycle, M28); absent = a non-separate sub-unit. The RID (id) is the stable external handle. Unique among active units that have a code; set/corrected/cleared via PUT /units/{id}/code. */
    'code'?: string | null;
    /** locale->text display name (all enabled locales; default-locale fallback + i18n store). */
    'name': { [key: string]: string };
    /** Descriptive instance label (e.g. battalion); never branched on in code. */
    'unitKind'?: string | null;
    /** Optional ordinal for sort/filter; never a PDP or shadow-gate input. */
    'level'?: number | null;
    'visibility': Visibility;
    'state': UnitState;
    /** Free-form long-tail attributes (JSONB). */
    'metadata'?: any | null;
    'createdAt': string;
    'updatedAt': string;
}
