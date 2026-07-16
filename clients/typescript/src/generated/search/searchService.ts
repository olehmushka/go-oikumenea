import { ISearchResultPage } from "./searchResultPage";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Unified cross-type object search (D-UnifiedSearch). Requires only an authenticated subject;
 * authorization is per-provider (read-permission gate) + per-row (visibility trim).
 *
 */
export interface ISearchService {
    /**
     * Search all (or the filtered subset of) registered object types for `query`. Each selected
     * provider contributes up to perTypeLimit hits per page; pageToken continues every
     * non-exhausted provider's keyset.
     *
     */
    searchObjects(query: string, types?: string | null, perTypeLimit?: number | null, pageSize?: number | null, pageToken?: string | null): Promise<ISearchResultPage>;
}

export class SearchService implements ISearchService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /**
     * Search all (or the filtered subset of) registered object types for `query`. Each selected
     * provider contributes up to perTypeLimit hits per page; pageToken continues every
     * non-exhausted provider's keyset.
     *
     */
    public searchObjects(query: string, types?: string | null, perTypeLimit?: number | null, pageSize?: number | null, pageToken?: string | null): Promise<ISearchResultPage> {
        return this.bridge.call<ISearchResultPage>(
            "SearchService",
            "searchObjects",
            "GET",
            "/search/v1/objects",
            __undefined,
            __undefined,
            {
                "query": query,
                "types": types,
                "perTypeLimit": perTypeLimit,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }
}
