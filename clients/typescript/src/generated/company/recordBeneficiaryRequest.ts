export interface IRecordBeneficiaryRequest {
    'personId': string;
    'ultimatePct'?: number | "NaN" | null;
    'declared'?: boolean | null;
}
