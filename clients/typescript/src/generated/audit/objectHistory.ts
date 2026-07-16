import { IObjectHistoryEvent } from "./objectHistoryEvent";

/**
 * A paginated, reverse-chronological history of one object, read from the audit ledger
 * (D-Temporal tier b, R-31). Serves the "what did this record say / who changed it when"
 * question a Gotham-style dossier timeline needs.
 *
 */
export interface IObjectHistory {
    /** The object RID whose history this is (the audit target_id filter). */
    'rid': string;
    'events': Array<IObjectHistoryEvent>;
    'nextPageToken'?: string | null;
    /**
     * True when this response withheld before/after payloads because the caller lacks the
     * sensitive-reader capability. The timeline itself is unaffected.
     *
     */
    'redacted': boolean;
}
