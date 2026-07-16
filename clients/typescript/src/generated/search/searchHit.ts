/** One matched object, type-erased down to what every object shares. */
export interface ISearchHit {
    /** The object's RID (canonical UUID text; self-describes service/kind/type). */
    'rid': string;
    /** The ontology registry token for the hit's type (e.g. person, languoid, company). */
    'objectType': string;
    /** Primary display line (person display name, catalog name, organization name, …). */
    'label': string;
    /** Optional secondary line (code, MGRS, title detail) — provider-chosen. */
    'snippet'?: string | null;
}
