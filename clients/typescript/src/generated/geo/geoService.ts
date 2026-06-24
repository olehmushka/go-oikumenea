import { ICountryList } from "./countryList";
import { IPlaceList } from "./placeList";
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
    /**
     * List active administrative places in the WOF gazetteer for a country, filtered by placetype
     * (default `region`), in name order. Powers region pickers (e.g. vehicle plate region).
     *
     */
    listPlaces(country: string, placetype?: string | null): Promise<IPlaceList>;
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

    /**
     * List active administrative places in the WOF gazetteer for a country, filtered by placetype
     * (default `region`), in name order. Powers region pickers (e.g. vehicle plate region).
     *
     */
    public listPlaces(country: string, placetype?: string | null): Promise<IPlaceList> {
        return this.bridge.call<IPlaceList>(
            "GeoService",
            "listPlaces",
            "GET",
            "/geo/v1/places",
            __undefined,
            __undefined,
            {
                "country": country,
                "placetype": placetype,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }
}
