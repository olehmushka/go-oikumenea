/** Record a stake held in this company (person or company holder). */
export interface IRecordShareholdingRequest {
    'holderKind': string;
    'holderId': string;
    'stakePct'?: number | "NaN" | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
}
