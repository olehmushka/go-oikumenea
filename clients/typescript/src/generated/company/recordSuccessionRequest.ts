/** Record that this company (the predecessor) is succeeded by the given successor. */
export interface IRecordSuccessionRequest {
    'successorId': string;
    'kind'?: string | null;
    'effectiveOn'?: string | null;
}
