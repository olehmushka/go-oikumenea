/** Update a custom role's name/description and (when present) replace its permission set. code/isBase immutable. */
export interface IUpdateRoleRequest {
    'name'?: string | null;
    'description'?: string | null;
    /** When present, fully replaces the permission set. Omit to leave permissions unchanged. */
    'permissions'?: Array<string> | null;
}
