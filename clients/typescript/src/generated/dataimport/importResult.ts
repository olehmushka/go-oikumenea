/** The per-object-type outcome of one idempotent upsert. */
export interface IImportResult {
    'objectType': string;
    /** Rows newly inserted. */
    'created': number;
    /** Existing rows whose values changed. */
    'updated': number;
    /** Rows already up to date (idempotent no-ops). */
    'skipped': number;
}
