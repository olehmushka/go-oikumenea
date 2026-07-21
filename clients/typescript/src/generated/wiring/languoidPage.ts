import { ILanguoidEntry } from "./languoidEntry";

/** The languoid catalog is ~27k rows; reads are keyset-paginated. */
export interface ILanguoidPage {
    'languoids': Array<ILanguoidEntry>;
    'nextPageToken'?: string | null;
}
