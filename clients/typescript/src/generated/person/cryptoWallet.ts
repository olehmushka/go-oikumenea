/**
 * A crypto-wallet attribution for a person (D-PersonOverlays, M35) — an Object. The address is
 * public on-chain data, but attributing it to a person is pii:sensitive; synergy with the M34
 * sanctioned-wallet cross-check. Hard-erased on purge.
 *
 */
export interface ICryptoWallet {
    'id': string;
    'personId': string;
    /** The on-chain wallet address. */
    'address': string;
    /** One of bitcoin | ethereum | solana | tron | bnb | polygon | monero | other. */
    'chain': string;
    /** One of exchange_kyc | blockchain_analysis | self_declared | leak | public_post | other. */
    'attributionMethod': string;
    /** Last-known approximate USD balance. */
    'balanceUsdApprox'?: number | "NaN" | null;
    'firstSeen'?: string | null;
    'lastSeen'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
