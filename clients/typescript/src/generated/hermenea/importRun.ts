/** One map+load lineage record. */
export interface IImportRun {
    'id': string;
    'sourceCode': string;
    'sourceVersion'?: string | null;
    /** running | succeeded | failed. */
    'status': string;
    'created': number;
    'updated': number;
    'skipped': number;
    'error'?: string | null;
    'startedAt': string;
    'finishedAt'?: string | null;
}
