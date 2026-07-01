/** A tattoo/scar/piercing/birthmark (D-PhysicalIdentity, M31). pii:special ceiling — a mark can reveal Art. 9 data. */
export interface IDistinguishingMark {
    'id': string;
    'personId': string;
    /** One of tattoo | scar | piercing | birthmark. */
    'kind': string;
    /** Where on the body (e.g. left forearm); free text. */
    'bodyLocation'?: string | null;
    /** The mark's appearance; free text. */
    'description'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
