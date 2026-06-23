/** Record or replace a guardian→ward link between the path person and counterpartId; role names the path person's side. */
export interface IUpsertGuardianshipRequest {
    'id'?: string | null;
    /** The other person's URN RID. */
    'counterpartId': string;
    /** The path person's role — guardian | ward. */
    'role': string;
    'relationCode'?: string | null;
    /** active | ended; defaults to active. */
    'status'?: string | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
}
