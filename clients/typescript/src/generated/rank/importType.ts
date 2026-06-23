import { IImportRank } from "./importRank";

/** A type node; carry EITHER children OR ranks (ranks live on leaf types only). */
export interface IImportType {
    'code': string;
    'name': string;
    'sortOrder'?: number | null;
    /** Nested child types; empty for a leaf type. */
    'children': Array<IImportType>;
    /** Ranks on this (leaf) type; empty for a non-leaf type. */
    'ranks': Array<IImportRank>;
}
