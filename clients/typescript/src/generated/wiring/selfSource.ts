/**
 * One of the connector's sources with its CURSOR — the most recent sync run, if any. A connector
 * reads this to resume: `latestRun.externalRunId` is where it last got to, `state` whether that
 * run finished.
 *
 */
export interface ISelfSource {
    'id': string;
    'code': string;
    'objectType'?: string | null;
    'enabled': boolean;
    'latestRunState'?: string | null;
    'latestExternalRunId'?: string | null;
    'latestFinishedAt'?: string | null;
}
