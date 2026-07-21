/** One source in a registration payload. */
export interface ISourceDeclaration {
    'code': string;
    'name': string;
    'objectType'?: string | null;
    'schedule'?: string | null;
    /** Defaults to true when absent. */
    'enabled'?: boolean | null;
}
