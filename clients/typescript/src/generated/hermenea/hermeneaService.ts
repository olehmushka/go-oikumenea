import { IImportRun } from "./importRun";
import { IImportSource } from "./importSource";
import { IJobRef } from "./jobRef";
import { IWatchlistQuery } from "./watchlistQuery";
import { IWatchlistResult } from "./watchlistResult";
import { IWorkerJob } from "./workerJob";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/** The ingestion/scheduler companion's control + read API. */
export interface IHermeneaService {
    /** Enqueue a sync job for a registered source (the push trigger from oikumenea). */
    triggerSync(source: string): Promise<IJobRef>;
    /** List the registered import sources. */
    listSources(): Promise<Array<IImportSource>>;
    /** List import-run lineage (most recent first). */
    listRuns(): Promise<Array<IImportRun>>;
    /** List worker jobs (most recent first). */
    listJobs(): Promise<Array<IWorkerJob>>;
    /**
     * Run a live watchlist screening check (D-Watchlists, M34): the first SYNCHRONOUS oikumenea→hermenea
     * call. Hermenea owns the outbound egress to OFAC/EU/UN/INTERPOL and a ≤24h cache; only per-person
     * match metadata is returned. The real interpol.api.bund.dev connector ships; sanctions providers
     * are a documented pluggable stub.
     *
     */
    checkWatchlist(request: IWatchlistQuery): Promise<IWatchlistResult>;
}

export class HermeneaService implements IHermeneaService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Enqueue a sync job for a registered source (the push trigger from oikumenea). */
    public triggerSync(source: string): Promise<IJobRef> {
        return this.bridge.call<IJobRef>(
            "HermeneaService",
            "triggerSync",
            "POST",
            "/hermenea/v1/sync/{source}",
            __undefined,
            __undefined,
            __undefined,
            [
                source,
            ],
            __undefined,
            __undefined
        );
    }

    /** List the registered import sources. */
    public listSources(): Promise<Array<IImportSource>> {
        return this.bridge.call<Array<IImportSource>>(
            "HermeneaService",
            "listSources",
            "GET",
            "/hermenea/v1/sources",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List import-run lineage (most recent first). */
    public listRuns(): Promise<Array<IImportRun>> {
        return this.bridge.call<Array<IImportRun>>(
            "HermeneaService",
            "listRuns",
            "GET",
            "/hermenea/v1/runs",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List worker jobs (most recent first). */
    public listJobs(): Promise<Array<IWorkerJob>> {
        return this.bridge.call<Array<IWorkerJob>>(
            "HermeneaService",
            "listJobs",
            "GET",
            "/hermenea/v1/jobs",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Run a live watchlist screening check (D-Watchlists, M34): the first SYNCHRONOUS oikumenea→hermenea
     * call. Hermenea owns the outbound egress to OFAC/EU/UN/INTERPOL and a ≤24h cache; only per-person
     * match metadata is returned. The real interpol.api.bund.dev connector ships; sanctions providers
     * are a documented pluggable stub.
     *
     */
    public checkWatchlist(request: IWatchlistQuery): Promise<IWatchlistResult> {
        return this.bridge.call<IWatchlistResult>(
            "HermeneaService",
            "checkWatchlist",
            "POST",
            "/hermenea/v1/watchlist/check",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }
}
