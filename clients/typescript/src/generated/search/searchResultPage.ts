import { ISearchHit } from "./searchHit";

/** Hits grouped by object type in fixed lexicographic type order. */
export interface ISearchResultPage {
    'hits': Array<ISearchHit>;
    /** Opaque composite keyset token; absent when every selected provider is exhausted. */
    'nextPageToken'?: string | null;
}
