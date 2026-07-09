/** Add a crypto wallet, or replace one when id is supplied (D-PersonOverlays, M35). */
export interface IUpsertCryptoWalletRequest {
    'id'?: string | null;
    'address': string;
    'chain'?: string | null;
    'attributionMethod'?: string | null;
    'balanceUsdApprox'?: number | "NaN" | null;
    'firstSeen'?: string | null;
    'lastSeen'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
