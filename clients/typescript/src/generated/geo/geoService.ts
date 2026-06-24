import { ICoordinateResolution } from "./coordinateResolution";
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
    /**
     * Reverse-geocode a WGS84 coordinate to the containing country plus the nearest place
     * (a locality — city/town/village — when one exists, else the nearest county/region).
     * Powers the locations form's auto-prefill after a coordinate is entered.
     *
     */
    resolveCoordinate(lat: number | "NaN", lng: number | "NaN"): Promise<ICoordinateResolution>;
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

    /**
     * Reverse-geocode a WGS84 coordinate to the containing country plus the nearest place
     * (a locality — city/town/village — when one exists, else the nearest county/region).
     * Powers the locations form's auto-prefill after a coordinate is entered.
     *
     */
    public resolveCoordinate(lat: number | "NaN", lng: number | "NaN"): Promise<ICoordinateResolution> {
        return this.bridge.call<ICoordinateResolution>(
            "GeoService",
            "resolveCoordinate",
            "GET",
            "/geo/v1/resolve",
            __undefined,
            __undefined,
            {
                "lat": lat,
                "lng": lng,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }
}
