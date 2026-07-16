import { INeighborhood } from "./neighborhood";
import { IObjectLinks } from "./objectLinks";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Generic object-link traversal (D-LinkTraversal). Requires only an authenticated subject;
 * authorization is per-link-arm (read-permission gate) + per-row (neighbor visibility trim).
 *
 */
export interface ILinkService {
    /**
     * Return every registered link incident to `rid`, grouped by (link type, direction). Each
     * arm contributes up to pageSize rows; pageToken continues every non-exhausted arm's keyset.
     *
     */
    getObjectLinks(rid: string, linkTypes?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IObjectLinks>;
    /**
     * Depth-1 neighborhood of `rid` as a flat neighbor list (the graph-explorer shape). Same
     * engine as getObjectLinks, flattened; depth>1 is out of scope (review-2026-09 Phase 15).
     *
     */
    searchAround(rid: string, linkTypes?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<INeighborhood>;
}

export class LinkService implements ILinkService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /**
     * Return every registered link incident to `rid`, grouped by (link type, direction). Each
     * arm contributes up to pageSize rows; pageToken continues every non-exhausted arm's keyset.
     *
     */
    public getObjectLinks(rid: string, linkTypes?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IObjectLinks> {
        return this.bridge.call<IObjectLinks>(
            "LinkService",
            "getObjectLinks",
            "GET",
            "/links/v1/objects/{rid}/links",
            __undefined,
            __undefined,
            {
                "linkTypes": linkTypes,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            [
                rid,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Depth-1 neighborhood of `rid` as a flat neighbor list (the graph-explorer shape). Same
     * engine as getObjectLinks, flattened; depth>1 is out of scope (review-2026-09 Phase 15).
     *
     */
    public searchAround(rid: string, linkTypes?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<INeighborhood> {
        return this.bridge.call<INeighborhood>(
            "LinkService",
            "searchAround",
            "GET",
            "/links/v1/objects/{rid}/search-around",
            __undefined,
            __undefined,
            {
                "linkTypes": linkTypes,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            [
                rid,
            ],
            __undefined,
            __undefined
        );
    }
}
