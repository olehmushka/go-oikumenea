import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listCards` returns under the same filters (M58 /
 * D-ObjectFacets). As with accounts, there is NO facet over the PAN — it is envelope-encrypted
 * and inside PCI-DSS CDE scope. `bin` and `lastFour` are clear, but they are display columns
 * for identifying one card, not distributions to group a registry by.
 *
 */
export interface ICardStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
