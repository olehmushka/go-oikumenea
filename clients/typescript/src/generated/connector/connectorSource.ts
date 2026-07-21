/**
 * One dataset a connector syncs, as reported by that connector. A read model: the connector
 * stays authoritative for execution, because scheduling lives in the connector.
 *
 */
export interface IConnectorSource {
    'id': string;
    'connectorId': string;
    'code': string;
    'name': string;
    /**
     * The core import target for push-mode sources (e.g. geo-places). Absent for sources that
     * only feed the connector's own lookups.
     *
     */
    'objectType'?: string | null;
    /** The connector's own schedule string, verbatim, for display. The core never parses it. */
    'schedule'?: string | null;
    'enabled': boolean;
}
