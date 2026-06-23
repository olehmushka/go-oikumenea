/**
 * Set or clear the person's rank in ONE rank system (one rank per system — D-Rank). The rank's
 * system is derived from the rank itself, so on set only rankId is needed; on clear, systemId
 * names the system to clear.
 *
 */
export interface ISetRankRequest {
    /** The URN RID of the rank to assign (its rank system is derived); omit to clear the person's rank in `systemId`. */
    'rankId'?: string | null;
    /** Required only when clearing (rankId omitted) — the URN RID of the rank system to clear. Ignored when rankId is present. */
    'systemId'?: string | null;
}
