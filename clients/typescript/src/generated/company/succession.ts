/** M&A / reorganization lineage — predecessor SUCCEEDED_BY successor (link__succeeded_by). */
export interface ISuccession {
    'id': string;
    'predecessorId': string;
    'predecessorLabel'?: string | null;
    'successorId': string;
    'successorLabel'?: string | null;
    /** One of merger | reorganization | rename | acquisition | spinoff. */
    'kind': string;
    'effectiveOn'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
