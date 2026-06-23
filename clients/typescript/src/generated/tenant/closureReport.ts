/** The result of a closure verify/rebuild for one graph (D-ClosureIntegrity / D-ClosureDriftHealth). */
export interface IClosureReport {
    /** The graph code this report covers. */
    'graph': string;
    /** Closure rows the recompute found missing vs. the stored closure. */
    'missingCount': number;
    /** Spurious stored closure rows the recompute did not produce. */
    'extraCount': number;
    'inDrift': boolean;
    /** Optional small drift sample for diagnostics. */
    'sample'?: any | null;
}
