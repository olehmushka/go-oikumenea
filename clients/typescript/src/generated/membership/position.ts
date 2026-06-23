import { IMembership } from "./membership";

/**
 * A unit-owned billet (D-Position) — an Object that exists whether or not anyone fills it (a
 * VACANCY is an active position with no active filling). Position grants no authority. The
 * current holder is populated by getPosition and by listPositions (null when vacant).
 *
 */
export interface IPosition {
    /** The position's URN RID (carried as a plain string). */
    'id': string;
    /** The URN RID of the owning unit. */
    'unitId': string;
    /** Stable, locale-agnostic identifier, unique within the unit; immutable by convention. */
    'code': string;
    /** The billet title as a locale -> text map (default-locale fallback + i18n translations). */
    'title': { [key: string]: string };
    /** Optional advisory establishment rank (a rank RID); never enforced against any filler. */
    'requiredRankId'?: string | null;
    /** One of active | abolished (abolish is a reversible status flip). */
    'status': string;
    /** App-managed display order within the unit. */
    'sortOrder'?: number | null;
    /** The current active filling, if any. Populated by getPosition and listPositions; null when vacant. */
    'holder'?: IMembership | null;
    'createdAt': string;
    'updatedAt': string;
}
