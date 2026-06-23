/** A natural person who is the ultimate beneficial owner of a company (link__beneficiary_of, UBO). */
export interface IBeneficiary {
    'id': string;
    'companyId': string;
    'companyLabel'?: string | null;
    'personId': string;
    'ultimatePct'?: number | "NaN" | null;
    /** Whether the ultimate ownership is registry-declared (vs computed). */
    'declared': boolean;
    'createdAt': string;
    'updatedAt': string;
}
