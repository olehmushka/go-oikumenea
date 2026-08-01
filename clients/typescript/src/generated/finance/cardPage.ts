import { ICard } from "./card";

/**
 * A keyset page of the instance-wide card registry (M58). Carries the same METADATA-ONLY
 * projection `listAccountCards` does — the PAN is never listed, only decrypted on `getCard`
 * for an authorized caller (PCI-DSS Req 3; D-DataScope CDE scope).
 *
 */
export interface ICardPage {
    'cards': Array<ICard>;
    'nextPageToken'?: string | null;
}
