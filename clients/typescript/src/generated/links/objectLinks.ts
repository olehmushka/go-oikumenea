import { ILinkGroup } from "./linkGroup";

/** The queried object's links, grouped by (link type, direction) in fixed order. */
export interface IObjectLinks {
    /** Echo of the queried object RID. */
    'rid': string;
    'groups': Array<ILinkGroup>;
    /** Opaque composite keyset token; absent when every selected link arm is exhausted. */
    'nextPageToken'?: string | null;
}
