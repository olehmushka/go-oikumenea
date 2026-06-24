import { ICountry } from "./country";
import { INearestPlace } from "./nearestPlace";

/** The reverse-geocode of a coordinate — the containing country plus the nearest place. Either may be absent if the gazetteer has no coverage at the point. */
export interface ICoordinateResolution {
    /** The country whose shape contains the point (or, lacking a shape, the nearest place's country); absent if neither resolves. */
    'country'?: ICountry | null;
    /** The nearest gazetteer place; absent only if the gazetteer is empty. */
    'place'?: INearestPlace | null;
}
