/**
 * A standardized cross-system comparability node (NATO STANAG 2116): the seeded reference catalog
 * two ranks compare through. Equivalence = same code; seniority = tier then ordinal.
 *
 */
export interface IRankGrade {
    /** STANAG 2116 grade code (e.g. OF-5, OR-9, OF(D)). */
    'code': string;
    /** One of officer | warrant | enlisted (enlisted < warrant < officer for cross-tier seniority). */
    'tier': string;
    /** Order within the tier (junior -> senior). */
    'ordinal': number;
    /** Generic, nation-neutral grade label (not translatable; grades are reference data, not i18n entities). */
    'name': string;
}
