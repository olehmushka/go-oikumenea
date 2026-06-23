/** A holder OWNS_STAKE in a company (link__owns_stake). Polymorphic holder; company-holder edges form the ownership DAG. */
export interface IShareholding {
    'id': string;
    'companyId': string;
    'companyLabel'?: string | null;
    'holderKind': string;
    'holderId': string;
    'holderLabel'?: string | null;
    'stakePct'?: number | "NaN" | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
