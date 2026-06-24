import { IPlace } from "./place";

/** The places matching the query, in name order. */
export interface IPlaceList {
    'places': Array<IPlace>;
}
