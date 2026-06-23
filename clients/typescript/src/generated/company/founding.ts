/** Who FOUNDED a company (link__founded). The founder is a person or a company (polymorphic holder). */
export interface IFounding {
    'id': string;
    'companyId': string;
    'companyLabel'?: string | null;
    /** One of person | company. */
    'holderKind': string;
    'holderId': string;
    /** Best-effort display label (company legal name for company holders; empty for persons). */
    'holderLabel'?: string | null;
    'foundedOn'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
