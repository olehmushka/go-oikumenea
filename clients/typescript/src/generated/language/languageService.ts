import { ILanguoid } from "./languoid";
import { ILanguoidList } from "./languoidList";
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
     * name/code substring. `limit` caps the page (default/clamped server-side) since the catalog is
     * large (~26k); narrow with the filters.
     *
     */
    listLanguages(level?: string | null, family?: string | null, parent?: string | null, topLevel?: boolean | null, query?: string | null, limit?: number | null, pageToken?: string | null): Promise<ILanguoidList>;
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
     * name/code substring. `limit` caps the page (default/clamped server-side) since the catalog is
     * large (~26k); narrow with the filters.
     *
     */
    public listLanguages(level?: string | null, family?: string | null, parent?: string | null, topLevel?: boolean | null, query?: string | null, limit?: number | null, pageToken?: string | null): Promise<ILanguoidList> {
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
                "parent": parent,
                "topLevel": topLevel,
                "query": query,
                "limit": limit,
                "pageToken": pageToken,
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
