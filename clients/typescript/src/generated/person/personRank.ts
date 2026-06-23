/** One rank a person holds, scoped to a rank system (the HOLDS_RANK link; one per system — D-Rank). A directory attribute, never an authz input. */
export interface IPersonRank {
    /** The URN RID of the rank system (derived from the rank); clients resolve the label via RankService. */
    'systemId': string;
    /** The URN RID of the rank held in that system. */
    'rankId': string;
}
