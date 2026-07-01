/** An effective-dated physical description (D-PhysicalIdentity, M31). pii:basic. */
export interface IPhysicalDescription {
    'id': string;
    'personId': string;
    /** Height in centimetres (1–299). */
    'heightCm'?: number | null;
    /** Weight in whole kilograms (1–699). */
    'weightKg'?: number | null;
    /** Eye color — a platform_colors RID (domain='eye', D-Color). Resolve label/hex via the platform color catalog. */
    'eyeColorId'?: string | null;
    /** Hair color — a platform_colors RID (domain='hair', D-Color). Resolve label/hex via the platform color catalog. */
    'hairColorId'?: string | null;
    /** Free-text physique (slim | athletic | heavy | ...). */
    'build'?: string | null;
    /** One of A+|A-|B+|B-|AB+|AB-|O+|O-|unknown. */
    'bloodType'?: string | null;
    /** ISO-8601 date the description took effect. */
    'effectiveFrom': string;
    /** ISO-8601 date it ceased; null = current. */
    'effectiveTo'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
