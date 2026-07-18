export interface IGrantPrincipalPermissionRequest {
    'principalId': string;
    'permission': string;
    /** Omit for an instance-wide grant; set to confine the machine to one organization. */
    'orgId'?: string | null;
}
