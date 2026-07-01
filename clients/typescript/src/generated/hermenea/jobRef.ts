/** A reference to an enqueued worker job (the result of a sync trigger). */
export interface IJobRef {
    'jobId': string;
    'status': string;
}
