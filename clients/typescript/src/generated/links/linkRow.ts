/** One reified link incident to the queried object, type-erased to what every link shares. */
export interface ILinkRow {
    /** The reified link's own RID (kind=link; self-describes which link type this is). */
    'linkRid': string;
    /** The neighbor object's RID at the other end of the link (canonical UUID text). */
    'targetRid': string;
    /** The ontology registry token for the neighbor's type (person, unit, organization, …). */
    'targetType': string;
    /** Best-effort neighbor display name as a locale→text map (D-i18n: all locales in every response, no negotiation); absent ⇒ the client falls back to the RID tail. Resolved server-side by the neighbor type's registered labeler. */
    'targetLabel'?: { [key: string]: string } | null;
    /** out (queried object is the source end), in (the target end), or peer (symmetric link). */
    'direction': string;
    /** Optional link attributes chosen by the descriptor (status, role, effective dates, …). */
    'attrs'?: { [key: string]: string } | null;
    /** Distance from the queried object: 1 (direct neighbor) or 2 (second hop, reached via viaRid). Only depth-2 search-around ever sets it; absent ⇒ a direct link. */
    'hop'?: number | null;
    /** For a hop-2 row, the hop-1 neighbor RID this row was reached through (canonical UUID text) — the intermediate node on the path. Absent for direct (hop-1) rows. */
    'viaRid'?: string | null;
}
