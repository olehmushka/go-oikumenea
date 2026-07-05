/** Add a regulatory sanction, or replace one when id is supplied (D-Watchlists, M34). */
export interface IUpsertRegulatorySanctionRequest {
    'id'?: string | null;
    'regulator': string;
    'actionType'?: string | null;
    'amount'?: number | "NaN" | null;
    'currency'?: string | null;
    'status'?: string | null;
    'sanctionDate'?: string | null;
    'sourceUrl'?: string | null;
    'externalId'?: string | null;
    'legalBasis'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
