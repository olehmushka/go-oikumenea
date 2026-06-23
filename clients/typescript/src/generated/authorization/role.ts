/** A named set of permission codes (Object Role). Base roles are seeded and immutable. */
export interface IRole {
    /** The role's URN RID (carried as a plain string). */
    'id': string;
    /** Stable, locale-agnostic identifier; immutable by convention. */
    'code': string;
    /** The role name as a locale -> text map (default-locale fallback + i18n translations). */
    'name': { [key: string]: string };
    /** The role description as a locale -> text map; empty when unset. */
    'description': { [key: string]: string };
    /** The role's permission codes (the closed catalog; code-defined). */
    'permissions': Array<string>;
    /** True for seeded base roles (immutable by instance admins). */
    'isBase': boolean;
    'createdAt': string;
    'updatedAt': string;
}
