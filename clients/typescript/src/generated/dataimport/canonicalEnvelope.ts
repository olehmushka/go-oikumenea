/**
 * The interchange document hermenea produces and oikumenea upserts. `objectType` must equal the
 * path object-type. `records` are object-type-specific JSON objects the registered handler reads.
 *
 */
export interface ICanonicalEnvelope {
    /** The importable object-type key (e.g. geo-countries); must match the path parameter. */
    'objectType': string;
    /** Stable source identifier (e.g. iso-3166) — stamped as row provenance. */
    'source': string;
    /** The source's version/edition (e.g. 2024) — stamped as row provenance. */
    'sourceVersion'?: string | null;
    /** License/attribution string carried for lineage (not persisted per-row). */
    'license'?: string | null;
    /** ISO-8601 timestamp the upstream snapshot was generated (lineage only). */
    'generatedAt'?: string | null;
    /** The object-type-specific records to upsert; each is a JSON object. */
    'records': Array<any>;
}
