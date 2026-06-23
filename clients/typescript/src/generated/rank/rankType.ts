import { IRank } from "./rank";

/**
 * A band within a category (e.g. officers, warrant, enlisted), ordered. Types form a TREE: a
 * type may nest under another type of the same category (parentTypeId), and ranks attach to
 * LEAF types only.
 *
 */
export interface IRankType {
    /** The type's URN RID (carried as a plain string). */
    'id': string;
    /** Stable, locale-agnostic identifier; unique among active siblings (same category + parent). */
    'code': string;
    /** locale->text display name. */
    'name': { [key: string]: string };
    /** Seniority ordinal among active siblings (within the parent type, or the category for a root type). */
    'sortOrder': number;
    /** The URN RID of the owning rank system (denormalized; equals the category's system). */
    'systemId': string;
    /** The URN RID of the owning rank category (the root category; carried on every type in the tree). */
    'categoryId': string;
    /** The URN RID of the parent type; absent for a root type of the category. */
    'parentTypeId'?: string | null;
    /** Child types in seniority order. Populated by getRankScheme; empty for a leaf type and on create/update. */
    'children': Array<IRankType>;
    /** The type's ranks in seniority order (leaf types only). Populated by getRankScheme; empty on create/update of the type. */
    'ranks': Array<IRank>;
}
