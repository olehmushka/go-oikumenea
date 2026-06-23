import { IRankType } from "./rankType";

/** A branch within a rank system (e.g. army, navy; or academic, administrative), ordered. */
export interface IRankCategory {
    /** The category's URN RID (carried as a plain string). */
    'id': string;
    /** Stable, locale-agnostic identifier; unique among active categories within the system. */
    'code': string;
    /** locale->text display name. */
    'name': { [key: string]: string };
    /** Seniority ordinal among active categories of the system. */
    'sortOrder': number;
    /** The URN RID of the owning rank system. */
    'systemId': string;
    /** The category's types in seniority order. Populated by getRankScheme; empty on create/update of the category. */
    'types': Array<IRankType>;
}
