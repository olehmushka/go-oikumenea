/** A specific grade within a type (e.g. sergeant, associate_professor), ordered for exact seniority. */
export interface IRank {
    /** The rank's URN RID (carried as a plain string). */
    'id': string;
    /** Stable, locale-agnostic identifier (D-Code); unique within its type among active ranks. */
    'code': string;
    /** locale->text display name (all enabled locales; default-locale fallback + i18n store). */
    'name': { [key: string]: string };
    /** Optional short form (e.g. SGT); locale-agnostic. */
    'abbreviation'?: string | null;
    /**
     * Optional standardized cross-system grade (NATO STANAG 2116; one of the GET /rank-grades
     * codes). Two ranks are equivalent across systems when they share a gradeCode; absent => no
     * cross-system comparison.
     *
     */
    'gradeCode'?: string | null;
    /** Seniority ordinal among active siblings within the type (lower = more junior). */
    'sortOrder': number;
    /** The URN RID of the owning rank system (denormalized; equals the type's system). */
    'systemId': string;
    /** The URN RID of the owning rank type. */
    'typeId': string;
}
