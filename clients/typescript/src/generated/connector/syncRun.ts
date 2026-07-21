/**
 * One reported execution of a source. The counts are the CONNECTOR's account of its work — the
 * core stores what it is told and does not recompute them.
 *
 */
export interface ISyncRun {
    'id': string;
    'sourceId': string;
    /**
     * The connector's own run id (hermenea's import_runs.id — the same value the M49 chunked
     * envelopes carry as `runId`), so a run correlates with the connector's ledger and with
     * per-row import provenance.
     *
     */
    'externalRunId'?: string | null;
    /** running | succeeded | failed. */
    'state': string;
    'created': number;
    'updated': number;
    'skipped': number;
    'error'?: string | null;
    'startedAt': string;
    'finishedAt'?: string | null;
}
