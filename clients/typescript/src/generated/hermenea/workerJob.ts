/** One unit of queued work in hermenea's runtime. */
export interface IWorkerJob {
    'id': string;
    'jobType': string;
    'sourceCode'?: string | null;
    /** queued | running | succeeded | failed | dead. */
    'status': string;
    'attempts': number;
    'maxAttempts': number;
    'runAfter': string;
    'lastError'?: string | null;
}
