/**
 * A structured GDPR lawful basis (D-OverlayFoundation, M29) — an Article 6 lawful basis or an
 * Article 9 special-category condition. Referenced by FK from every future pii:special overlay
 * store; the `code` is the stable handle.
 *
 */
export interface ILegalBasisKind {
    'code': string;
    'name': string;
    /** art6 (lawful basis) | art9 (special-category condition). */
    'article': string;
    /** active | retired. */
    'status': string;
    'sortOrder'?: number | null;
}
