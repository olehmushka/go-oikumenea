/** Add a physical description, or replace one when id is supplied. */
export interface IUpsertPhysicalDescriptionRequest {
    /** The RID of an existing description row to replace; omit to add a new row. */
    'id'?: string | null;
    'heightCm'?: number | null;
    'weightKg'?: number | null;
    /** A platform_colors RID (domain='eye', D-Color). */
    'eyeColorId'?: string | null;
    /** A platform_colors RID (domain='hair', D-Color). */
    'hairColorId'?: string | null;
    'build'?: string | null;
    'bloodType'?: string | null;
    /** ISO-8601 date; defaults to today on insert. */
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
