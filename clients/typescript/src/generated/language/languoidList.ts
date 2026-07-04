import { ILanguoid } from "./languoid";

/** A page of languoids in code order. */
export interface ILanguoidList {
    'languoids': Array<ILanguoid>;
    /**
     * Keyset cursor (the last glottocode on this page) to fetch the next page; absent when the
     * page is the last one. Pass it back as `pageToken`. The filters must be held constant across
     * a paged sweep.
     *
     */
    'nextPageToken'?: string | null;
}
