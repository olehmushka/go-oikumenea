/** Record or replace a sponsor→sponsored link between the path person and counterpartId; role names the path person's side. relationCode is required (category=sponsorship). */
export interface IUpsertSponsorshipRequest {
    'id'?: string | null;
    /** The other person's URN RID. */
    'counterpartId': string;
    /** The path person's role — sponsor | sponsored. */
    'role': string;
    /** Required relation-type code (category=sponsorship). */
    'relationCode': string;
    /** active | ended; defaults to active. */
    'status'?: string | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    /** Optional education context — the enrollment (D-Education, M20) this sponsorship relates to. */
    'enrollmentId'?: string | null;
    /** Optional sponsor education role — one of professor | tutor | curator | advisor. */
    'educationRole'?: string | null;
}
