/** A registered external dataset hermenea can sync into oikumenea. */
export interface IImportSource {
    'code': string;
    'name': string;
    /** http | file. */
    'connectorType': string;
    /** The oikumenea import target (e.g. geo-countries). */
    'objectType': string;
    /** URL (http) or bundled path (file). */
    'locator': string;
    /** Cron spec; absent means trigger-only. */
    'cron'?: string | null;
    'enabled': boolean;
}
