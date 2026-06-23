/**
 * A coordinate in one of several supported formats. The application converts it to a canonical
 * WGS84 lat/lon, derives the MGRS, and preserves this input verbatim as the location's
 * sourceCoordinate. `format` selects which fields are read (the others are ignored); the
 * converter set is a pluggable registry, so more formats can be added without a schema change.
 *
 */
export interface ICoordinateInput {
    /** One of latlon | mgrs | utm | sk42 | sk42grid. */
    'format': string;
    /** WGS84 latitude (format=latlon). */
    'latitude'?: number | "NaN" | null;
    /** WGS84 longitude (format=latlon). */
    'longitude'?: number | "NaN" | null;
    /** MGRS grid reference, e.g. 36UUA2418291607 (format=mgrs). */
    'mgrs'?: string | null;
    /** UTM or СК-42 (Gauss-Krüger) zone number (format=utm|sk42). */
    'zone'?: number | null;
    /** N | S, the UTM hemisphere (format=utm). */
    'hemisphere'?: string | null;
    /** Easting in metres (format=utm|sk42). */
    'easting'?: number | "NaN" | null;
    /** Northing in metres (format=utm|sk42). */
    'northing'?: number | "NaN" | null;
    /** СК-42 grid-square reference (format=sk42grid). */
    'grid'?: string | null;
}
