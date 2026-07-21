/**
 * Report a run — opening it (`state: running`) and closing it (`succeeded`/`failed`) are the
 * same call. IDEMPOTENT on (source, externalRunId): a connector retrying its report updates the
 * run rather than duplicating it, which matters because connectors retry with backoff.
 *
 */
export interface IReportSyncRunRequest {
    /** The source's code within the CALLING connector — not a RID, so a connector needs no prior read. */
    'sourceCode': string;
    'externalRunId'?: string | null;
    'state': string;
    'created'?: number | null;
    'updated'?: number | null;
    'skipped'?: number | null;
    'error'?: string | null;
    'startedAt'?: string | null;
    'finishedAt'?: string | null;
}
