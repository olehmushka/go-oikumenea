/** A person's informal identifier / позивний (D-PersonContactChannels). pii:basic; unique per person among active. */
export interface ICallSign {
    'id': string;
    'personId': string;
    /** The call sign label (required; unique per person). */
    'callSign': string;
    /** The person's primary call sign (at most one active). */
    'isPrimary': boolean;
}
