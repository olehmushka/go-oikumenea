import { ICoordinateInput } from "./coordinateInput";

/** A shared place — a precise coordinate, an app-derived MGRS, and a structured postal address. */
export interface ILocation {
    /** The location's RID (location service); what owning modules reference by FK. */
    'id': string;
    /** WGS84 latitude of the authoritative coordinate. */
    'latitude': number | "NaN";
    /** WGS84 longitude of the authoritative coordinate. */
    'longitude': number | "NaN";
    /** App-derived MGRS grid reference (absent for polar UPS coordinates, out of scope). */
    'mgrs'?: string | null;
    /** The coordinate as originally supplied (its format + raw values), preserved verbatim. */
    'sourceCoordinate'?: ICoordinateInput | null;
    /** The country's RID (resolve an ISO alpha-2 code via GET /geo/countries). */
    'countryId': string;
    'adminArea1'?: string | null;
    'adminArea2'?: string | null;
    'locality'?: string | null;
    'street'?: string | null;
    'houseNumber'?: string | null;
    'postalCode'?: string | null;
    'rawAddress'?: string | null;
    /** Optional place-type classification (location_location_types RID). */
    'typeId'?: string | null;
    /** Localized name (locale->text) of the place type, when typeId is set. */
    'typeName'?: { [key: string]: string } | null;
    'createdAt': string;
    'updatedAt': string;
}
