import { ILocation } from "./location";
import { ILocationPage } from "./locationPage";
import { ILocationTypeList } from "./locationTypeList";
import { ILocationWrite } from "./locationWrite";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * CRUD + spatial queries over the shared location entity (D-Location). A location has no unit scope
 * of its own; reads/writes are satisfied anywhere via the PEP (location.read/create/update,
 * location.types.manage). The coordinate may be supplied in several formats; the application converts
 * it to WGS84 and derives the MGRS on every coordinate change. Radius/bbox use PostGIS ST_DWithin.
 *
 */
export interface ILocationService {
    /** Create a location from a coordinate (any supported format) + address; derives MGRS. Location:CoordinateRequired when the coordinate is missing. */
    createLocation(request: ILocationWrite): Promise<ILocation>;
    /** Read one location by its RID. Location:LocationNotFound when absent. */
    getLocation(locationId: string): Promise<ILocation>;
    /** Replace a location's coordinate/address/type (re-derives MGRS on the coordinate). */
    updateLocation(locationId: string, request: ILocationWrite): Promise<ILocation>;
    /** Soft-delete a location. Location:LocationInUse when an owner still references it. */
    deleteLocation(locationId: string): Promise<void>;
    /**
     * Spatial or text query, token-paginated. Supply a `query` (case-insensitive match on the
     * address fields — no spatial window required), or a radius (lat + lng + radiusM, via
     * ST_DWithin), or a bounding box (minLat + minLng + maxLat + maxLng); Location:QueryWindowRequired
     * when none is given.
     *
     */
    listLocations(lat?: number | "NaN" | null, lng?: number | "NaN" | null, radiusM?: number | "NaN" | null, minLat?: number | "NaN" | null, minLng?: number | "NaN" | null, maxLat?: number | "NaN" | null, maxLng?: number | "NaN" | null, pageSize?: number | null, pageToken?: string | null, query?: string | null): Promise<ILocationPage>;
    /** List the active place-type catalog. */
    listLocationTypes(): Promise<ILocationTypeList>;
}

export class LocationService implements ILocationService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Create a location from a coordinate (any supported format) + address; derives MGRS. Location:CoordinateRequired when the coordinate is missing. */
    public createLocation(request: ILocationWrite): Promise<ILocation> {
        return this.bridge.call<ILocation>(
            "LocationService",
            "createLocation",
            "POST",
            "/location/v1/locations",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Read one location by its RID. Location:LocationNotFound when absent. */
    public getLocation(locationId: string): Promise<ILocation> {
        return this.bridge.call<ILocation>(
            "LocationService",
            "getLocation",
            "GET",
            "/location/v1/locations/{locationId}",
            __undefined,
            __undefined,
            __undefined,
            [
                locationId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Replace a location's coordinate/address/type (re-derives MGRS on the coordinate). */
    public updateLocation(locationId: string, request: ILocationWrite): Promise<ILocation> {
        return this.bridge.call<ILocation>(
            "LocationService",
            "updateLocation",
            "PUT",
            "/location/v1/locations/{locationId}",
            request,
            __undefined,
            __undefined,
            [
                locationId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Soft-delete a location. Location:LocationInUse when an owner still references it. */
    public deleteLocation(locationId: string): Promise<void> {
        return this.bridge.call<void>(
            "LocationService",
            "deleteLocation",
            "DELETE",
            "/location/v1/locations/{locationId}",
            __undefined,
            __undefined,
            __undefined,
            [
                locationId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Spatial or text query, token-paginated. Supply a `query` (case-insensitive match on the
     * address fields — no spatial window required), or a radius (lat + lng + radiusM, via
     * ST_DWithin), or a bounding box (minLat + minLng + maxLat + maxLng); Location:QueryWindowRequired
     * when none is given.
     *
     */
    public listLocations(lat?: number | "NaN" | null, lng?: number | "NaN" | null, radiusM?: number | "NaN" | null, minLat?: number | "NaN" | null, minLng?: number | "NaN" | null, maxLat?: number | "NaN" | null, maxLng?: number | "NaN" | null, pageSize?: number | null, pageToken?: string | null, query?: string | null): Promise<ILocationPage> {
        return this.bridge.call<ILocationPage>(
            "LocationService",
            "listLocations",
            "GET",
            "/location/v1/locations",
            __undefined,
            __undefined,
            {
                "lat": lat,
                "lng": lng,
                "radiusM": radiusM,
                "minLat": minLat,
                "minLng": minLng,
                "maxLat": maxLat,
                "maxLng": maxLng,
                "pageSize": pageSize,
                "pageToken": pageToken,
                "query": query,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List the active place-type catalog. */
    public listLocationTypes(): Promise<ILocationTypeList> {
        return this.bridge.call<ILocationTypeList>(
            "LocationService",
            "listLocationTypes",
            "GET",
            "/location/v1/location/types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }
}
