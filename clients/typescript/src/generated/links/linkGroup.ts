import { ILinkRow } from "./linkRow";

/** All links of one type + direction incident to the queried object. */
export interface ILinkGroup {
    /** The bare link-type name from the pkg/rid registry (e.g. member_of, kin_parent_of). */
    'linkType': string;
    /** The neighbor object type all rows in this group point at. */
    'targetType': string;
    'direction': string;
    'rows': Array<ILinkRow>;
}
