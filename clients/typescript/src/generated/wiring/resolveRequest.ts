/**
 * Resolve a batch of natural keys to RIDs within one catalog, in a single round trip — the
 * connector's real need (mapping many rows), not one code at a time. `catalog` is one of
 * `country` (ISO-3166-1 alpha-2), `languoid` (glottocode), `writing-system` (ISO-15924). The
 * legal-basis catalog is code-keyed and has no RID, so it is a catalog READ, not a resolve target.
 *
 */
export interface IResolveRequest {
    'catalog': string;
    'codes': Array<string>;
}
