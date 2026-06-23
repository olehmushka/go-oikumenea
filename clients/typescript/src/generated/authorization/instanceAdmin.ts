/** A person on the instance-wide authority plane (the reified link__instance_admin). */
export interface IInstanceAdmin {
    /** The grant's URN RID (carried as a plain string). */
    'id': string;
    'personId': string;
    /** The granting person RID; null for the install bootstrap grant. */
    'grantedBy'?: string | null;
    'grantedAt': string;
    'revokedAt'?: string | null;
    'revokedBy'?: string | null;
}
