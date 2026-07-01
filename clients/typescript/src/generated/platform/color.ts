/**
 * One entry in a per-domain color palette (D-Color). The per-domain `code` is the stable handle;
 * `name` is the translatable label (locale -> text map, D-i18n); `hex` is an optional
 * representative swatch (biological eye/hair colors are categories, not precise hex).
 *
 */
export interface IColor {
    'id': string;
    /** The palette this color belongs to — eye | hair | vehicle. */
    'domain': string;
    'code': string;
    /** The translatable label as a locale -> text map (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
    'hex'?: string | null;
    /** active | retired. */
    'status': string;
    'sortOrder'?: number | null;
}
