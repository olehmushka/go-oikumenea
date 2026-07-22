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
     * Neighborhood of `rid` as a flat neighbor list (the graph-explorer shape). Same engine as
     * getObjectLinks, flattened. depth=1 (default) returns direct neighbors; depth=2 additionally
     * expands each direct neighbor one more hop (rows tagged hop=2, carrying viaRid), walked
     * exhaustively via a keyset frontier. Per-hop authorization is identical to depth-1 (arm gate
     * + neighbor visibility trim at every hop); a neighbor the subject cannot read is neither
     * returned nor expanded.
     *
     */
    searchAround(rid: string, depth?: number | null, linkTypes?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<INeighborhood>;
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
     * Neighborhood of `rid` as a flat neighbor list (the graph-explorer shape). Same engine as
     * getObjectLinks, flattened. depth=1 (default) returns direct neighbors; depth=2 additionally
     * expands each direct neighbor one more hop (rows tagged hop=2, carrying viaRid), walked
     * exhaustively via a keyset frontier. Per-hop authorization is identical to depth-1 (arm gate
     * + neighbor visibility trim at every hop); a neighbor the subject cannot read is neither
     * returned nor expanded.
     *
     */
    public searchAround(rid: string, depth?: number | null, linkTypes?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<INeighborhood> {
        return this.bridge.call<INeighborhood>(
            "LinkService",
            "searchAround",
            "GET",
            "/links/v1/objects/{rid}/search-around",
            __undefined,
            __undefined,
            {
                "depth": depth,
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
