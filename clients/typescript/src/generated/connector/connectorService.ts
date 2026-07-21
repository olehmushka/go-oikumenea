import { IConnector } from "./connector";
import { IConnectorPage } from "./connectorPage";
import { IConnectorSourceList } from "./connectorSourceList";
import { IRegisterConnectorRequest } from "./registerConnectorRequest";
import { IReportSyncRunRequest } from "./reportSyncRunRequest";
import { ISyncRun } from "./syncRun";
import { ISyncRunPage } from "./syncRunPage";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * The connector-plane registry. Self-service surfaces (`registerConnector`, `reportSyncRun`) are for
 * MACHINE subjects and bind to the caller's own principal; the read surfaces are for operators.
 * Visibility, not orchestration — there is deliberately no endpoint to trigger, pause or retry a
 * run, because scheduling belongs to the connector (D-ConnectorPlane rejects core-side orchestration).
 *
 */
export interface IConnectorService {
    /**
     * Idempotent self-registration (`connector.register`, machine subjects only). The core binds the
     * row to the CALLING service principal; the request cannot name another. Replaces the declared
     * source set. Returns Connector:ConnectorConflict if the code belongs to a different principal.
     *
     */
    registerConnector(request: IRegisterConnectorRequest): Promise<IConnector>;
    /**
     * Report a sync run (`connector.report`, machine subjects only), for a source of the CALLING
     * connector. Idempotent on (source, externalRunId). Returns Connector:SourceNotFound if the
     * calling connector declares no such source.
     *
     */
    reportSyncRun(request: IReportSyncRunRequest): Promise<ISyncRun>;
    /** List registered connectors, token-paginated (`connector.read`). */
    listConnectors(pageSize?: number | null, pageToken?: string | null): Promise<IConnectorPage>;
    /** Fetch one connector (`connector.read`). */
    getConnector(connectorId: string): Promise<IConnector>;
    /** The sources a connector has declared (`connector.read`). */
    listConnectorSources(connectorId: string): Promise<IConnectorSourceList>;
    /**
     * Recent runs, newest first, token-paginated (`connector.read`). Filter by source when given,
     * otherwise the whole fleet's runs — the operator's "what has been happening" view.
     *
     */
    listSyncRuns(sourceId?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ISyncRunPage>;
}

export class ConnectorService implements IConnectorService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /**
     * Idempotent self-registration (`connector.register`, machine subjects only). The core binds the
     * row to the CALLING service principal; the request cannot name another. Replaces the declared
     * source set. Returns Connector:ConnectorConflict if the code belongs to a different principal.
     *
     */
    public registerConnector(request: IRegisterConnectorRequest): Promise<IConnector> {
        return this.bridge.call<IConnector>(
            "ConnectorService",
            "registerConnector",
            "PUT",
            "/connectors/v1/registration",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Report a sync run (`connector.report`, machine subjects only), for a source of the CALLING
     * connector. Idempotent on (source, externalRunId). Returns Connector:SourceNotFound if the
     * calling connector declares no such source.
     *
     */
    public reportSyncRun(request: IReportSyncRunRequest): Promise<ISyncRun> {
        return this.bridge.call<ISyncRun>(
            "ConnectorService",
            "reportSyncRun",
            "POST",
            "/connectors/v1/sync-runs",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List registered connectors, token-paginated (`connector.read`). */
    public listConnectors(pageSize?: number | null, pageToken?: string | null): Promise<IConnectorPage> {
        return this.bridge.call<IConnectorPage>(
            "ConnectorService",
            "listConnectors",
            "GET",
            "/connectors/v1/connectors",
            __undefined,
            __undefined,
            {
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Fetch one connector (`connector.read`). */
    public getConnector(connectorId: string): Promise<IConnector> {
        return this.bridge.call<IConnector>(
            "ConnectorService",
            "getConnector",
            "GET",
            "/connectors/v1/connectors/{connectorId}",
            __undefined,
            __undefined,
            __undefined,
            [
                connectorId,
            ],
            __undefined,
            __undefined
        );
    }

    /** The sources a connector has declared (`connector.read`). */
    public listConnectorSources(connectorId: string): Promise<IConnectorSourceList> {
        return this.bridge.call<IConnectorSourceList>(
            "ConnectorService",
            "listConnectorSources",
            "GET",
            "/connectors/v1/connectors/{connectorId}/sources",
            __undefined,
            __undefined,
            __undefined,
            [
                connectorId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Recent runs, newest first, token-paginated (`connector.read`). Filter by source when given,
     * otherwise the whole fleet's runs — the operator's "what has been happening" view.
     *
     */
    public listSyncRuns(sourceId?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ISyncRunPage> {
        return this.bridge.call<ISyncRunPage>(
            "ConnectorService",
            "listSyncRuns",
            "GET",
            "/connectors/v1/sync-runs",
            __undefined,
            __undefined,
            {
                "sourceId": sourceId,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }
}
