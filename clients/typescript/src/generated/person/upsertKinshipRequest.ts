/** Record or replace a parent→child kinship between the path person and counterpartId; role names the path person's side. */
export interface IUpsertKinshipRequest {
    'id'?: string | null;
    /** The other person's URN RID. */
    'counterpartId': string;
    /** The path person's role — parent | child. */
    'role': string;
    /** active | disestablished; defaults to active. */
    'status'?: string | null;
}
