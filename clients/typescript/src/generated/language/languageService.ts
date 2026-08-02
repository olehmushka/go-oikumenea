import { ILanguoid } from "./languoid";
import { ILanguoidList } from "./languoidList";
import { ILanguoidStats } from "./languoidStats";
import { IWritingSystemList } from "./writingSystemList";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Read-only lookup over the Glottolog languoid forest + ISO-15924 writing systems (D-Languages).
 * Reads are gated by `language.read`. Writes happen via the hermenea import pipeline, not here.
 *
 */
export interface ILanguageService {
    /**
     * List languoids in code order, optionally filtered by level, root family (glottocode), and a
     * name/code substring. `pageSize` caps the page (default/clamped server-side) since the catalog
     * is large (~26k); narrow with the filters.
     *
     * `pageSize` was named `limit` before M58 ticket 4. The rename is a WIRE BREAK, taken because
     * the paging-arg convention (`pageSize`/`pageToken`) is a checked invariant held by every other
     * faceted type, and this endpoint was the only holdout.
     *
     */
    listLanguages(level?: string | null, family?: string | null, macroarea?: string | null, status?: string | null, parent?: string | null, topLevel?: boolean | null, query?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ILanguoidList>;
    /**
     * Facet distributions over the languoid registry — the dashboard half of the languoid facet
     * vocabulary (M58 / D-ObjectFacets). Takes exactly the STRUCTURAL filter args `listLanguages`
     * takes — `level`, `family`, `macroarea`, `status` and the free-text `query` — plus the
     * `facets` CSV, so a dashboard and a list are two renderings of one request state.
     *
     * The tree-traversal args `parent`/`topLevel` have no counterpart here on purpose: they switch
     * the LIST to a one-level hierarchy walk rather than describing the registry, so there is
     * nothing for them to count.
     *
     * ONE arm, and no subject. The languoid catalog is instance-global reference data — no
     * row-level security, no unit column, no reach predicate — so `language.read` held anywhere is
     * the whole visibility decision and there is none left to fold into the count.
     *
     * The path is `/stats/languages` rather than `/languages/stats` because the server's router
     * rejects a literal path segment that is a sibling of `{id}` — see the route-conflict guard in
     * `internal/platform/transport`.
     *
     */
    languoidStats(facets?: string | null, level?: string | null, family?: string | null, macroarea?: string | null, status?: string | null, query?: string | null): Promise<ILanguoidStats>;
    /** Fetch one languoid by its RID. */
    getLanguage(id: string): Promise<ILanguoid>;
    /** List the ISO-15924 writing systems in code order. */
    listWritingSystems(): Promise<IWritingSystemList>;
}

export class LanguageService implements ILanguageService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /**
     * List languoids in code order, optionally filtered by level, root family (glottocode), and a
     * name/code substring. `pageSize` caps the page (default/clamped server-side) since the catalog
     * is large (~26k); narrow with the filters.
     *
     * `pageSize` was named `limit` before M58 ticket 4. The rename is a WIRE BREAK, taken because
     * the paging-arg convention (`pageSize`/`pageToken`) is a checked invariant held by every other
     * faceted type, and this endpoint was the only holdout.
     *
     */
    public listLanguages(level?: string | null, family?: string | null, macroarea?: string | null, status?: string | null, parent?: string | null, topLevel?: boolean | null, query?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ILanguoidList> {
        return this.bridge.call<ILanguoidList>(
            "LanguageService",
            "listLanguages",
            "GET",
            "/language/v1/languages",
            __undefined,
            __undefined,
            {
                "level": level,
                "family": family,
                "macroarea": macroarea,
                "status": status,
                "parent": parent,
                "topLevel": topLevel,
                "query": query,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Facet distributions over the languoid registry — the dashboard half of the languoid facet
     * vocabulary (M58 / D-ObjectFacets). Takes exactly the STRUCTURAL filter args `listLanguages`
     * takes — `level`, `family`, `macroarea`, `status` and the free-text `query` — plus the
     * `facets` CSV, so a dashboard and a list are two renderings of one request state.
     *
     * The tree-traversal args `parent`/`topLevel` have no counterpart here on purpose: they switch
     * the LIST to a one-level hierarchy walk rather than describing the registry, so there is
     * nothing for them to count.
     *
     * ONE arm, and no subject. The languoid catalog is instance-global reference data — no
     * row-level security, no unit column, no reach predicate — so `language.read` held anywhere is
     * the whole visibility decision and there is none left to fold into the count.
     *
     * The path is `/stats/languages` rather than `/languages/stats` because the server's router
     * rejects a literal path segment that is a sibling of `{id}` — see the route-conflict guard in
     * `internal/platform/transport`.
     *
     */
    public languoidStats(facets?: string | null, level?: string | null, family?: string | null, macroarea?: string | null, status?: string | null, query?: string | null): Promise<ILanguoidStats> {
        return this.bridge.call<ILanguoidStats>(
            "LanguageService",
            "languoidStats",
            "GET",
            "/language/v1/stats/languages",
            __undefined,
            __undefined,
            {
                "facets": facets,
                "level": level,
                "family": family,
                "macroarea": macroarea,
                "status": status,
                "query": query,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Fetch one languoid by its RID. */
    public getLanguage(id: string): Promise<ILanguoid> {
        return this.bridge.call<ILanguoid>(
            "LanguageService",
            "getLanguage",
            "GET",
            "/language/v1/languages/{id}",
            __undefined,
            __undefined,
            __undefined,
            [
                id,
            ],
            __undefined,
            __undefined
        );
    }

    /** List the ISO-15924 writing systems in code order. */
    public listWritingSystems(): Promise<IWritingSystemList> {
        return this.bridge.call<IWritingSystemList>(
            "LanguageService",
            "listWritingSystems",
            "GET",
            "/language/v1/writing-systems",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }
}
