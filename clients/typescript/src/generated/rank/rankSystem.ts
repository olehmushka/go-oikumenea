import { IRankCategory } from "./rankCategory";

/**
 * The top level of the scheme (D-RankSystems): a national/organizational rank ladder. One scheme
 * may hold several at once (a coalition directory); a single-nation deployment has one.
 *
 */
export interface IRankSystem {
    /** The system's URN RID (carried as a plain string). */
    'id': string;
    /** Stable, locale-agnostic identifier (e.g. us-armed-forces); unique among active systems. */
    'code': string;
    /** locale->text display name. */
    'name': { [key: string]: string };
    /** Order among active systems. */
    'sortOrder': number;
    /** Country RID of the national origin (resolve via GET /geo/countries); absent for a supranational system (NATO/UN). */
    'country'?: string | null;
    /** The system's categories in seniority order. Populated by getRankScheme; empty on create/update of the system. */
    'categories': Array<IRankCategory>;
}
