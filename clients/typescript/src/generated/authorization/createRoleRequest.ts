/** Create a custom role. name/description are the default-locale fallbacks (translatable separately). */
export interface ICreateRoleRequest {
    'code': string;
    'name': string;
    'description'?: string | null;
    /** Permission codes from the catalog; instance-scope and unknown codes are rejected. */
    'permissions': Array<string>;
}
