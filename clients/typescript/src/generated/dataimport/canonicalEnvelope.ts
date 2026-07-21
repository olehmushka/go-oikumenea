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
    /**
     * The owning organization RID when the target is ORG-OWNED operational data (M55 / the RLS
     * service arm): the import permission is then checked against this org, so an org-confined
     * connector imports only its own organization's data and a foreign-org grant is rejected.
     * Absent = an instance-wide reference catalog (the pre-M55 default), which demands an
     * instance-wide grant.
     *
     */
    'orgId'?: string | null;
    /** The object-type-specific records to upsert; each is a JSON object. */
    'records': Array<any>;
    /**
     * Hermenea's import-run identifier when the dataset arrives as a chunked sequence of
     * envelopes (R-05). Lineage/audit correlation only — oikumenea keeps no per-run state;
     * resumability is owned by the sender (last-acked seq on the hermenea job).
     *
     */
    'runId'?: string | null;
    /**
     * 1-based chunk index within a chunked run. Chunks are sent sequentially and each is
     * applied in its own transaction; replaying a chunk is safe (every record apply is a
     * natural-key idempotent upsert). Absent (with runId/isLast absent) = a single-shot
     * envelope, the pre-chunking semantics.
     *
     */
    'seq'?: number | null;
    /**
     * True on the final chunk of a chunked run; triggers the object-type's batch
     * finalizers (e.g. the languoid closure + family_code rebuild). The final chunk may
     * carry zero records (a pure finalize marker).
     *
     */
    'isLast'?: boolean | null;
}
