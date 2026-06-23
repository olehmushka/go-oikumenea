import { ICoordinateInput } from "./coordinateInput";

/**
 * Create/replace payload for a location. `coordinate` is the required spine (validated in the
 * application, not the wire, so a missing/unparseable coordinate returns Location:CoordinateRequired
 * or Location:CoordinateInvalid rather than a deserialization error). MGRS is never supplied — it
 * is app-derived from the resolved WGS84 coordinate.
 *
 */
export interface ILocationWrite {
    /** The coordinate in any supported format; required (absence → Location:CoordinateRequired). */
    'coordinate'?: ICoordinateInput | null;
    /** The country's RID (required). */
    'countryId': string;
    'adminArea1'?: string | null;
    'adminArea2'?: string | null;
    'locality'?: string | null;
    'street'?: string | null;
    'houseNumber'?: string | null;
    'postalCode'?: string | null;
    'rawAddress'?: string | null;
    'typeId'?: string | null;
}
