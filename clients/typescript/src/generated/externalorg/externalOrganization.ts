/**
 * An external organization at registry grade. Soft-deleted, not destroyed. The RID is the
 * external handle. Carries the D-OverlayFoundation provisional/resolved status + attribution.
 *
 */
export interface IExternalOrganization {
    'id': string;
    'kindId': string;
    /** Best-effort default-locale kind name. */
    'kindLabel'?: string | null;
    /** Default-locale fallback + i18n translations. */
    'name': { [key: string]: string };
    'code'?: string | null;
    'countryId'?: string | null;
    'countryLabel'?: string | null;
    /** Optional Wikidata Q-id concordance (the hermenea import natural key). */
    'wikidataId'?: string | null;
    /** One of provisional | resolved. */
    'status': string;
    /** One of self_declared | operator_verified | imported (attribution). */
    'source': string;
    /** One of confirmed | probable | possible (attribution). */
    'confidence': string;
    'asOf'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
