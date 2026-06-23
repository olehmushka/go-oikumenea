import { ICountryList } from "./countryList";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Read-only lookup over the country registry so clients can resolve a country to its RID. Reads
 * are gated by `country.read` once authorization (M7) is enforced.
 *
 */
export interface IGeoService {
    /** List the active countries in display order (sort_order, then code). */
    listCountries(): Promise<ICountryList>;
}

export class GeoService implements IGeoService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** List the active countries in display order (sort_order, then code). */
    public listCountries(): Promise<ICountryList> {
        return this.bridge.call<ICountryList>(
            "GeoService",
            "listCountries",
            "GET",
            "/geo/v1/countries",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }
}
