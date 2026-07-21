/** The calling connector's own registry row. */
export interface ISelfConnector {
    'id': string;
    'code': string;
    'name': string;
    'status': string;
    'lastSeenAt'?: string | null;
}
