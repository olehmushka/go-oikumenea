/** Add a holder (person or company) to the account; role defaults to primary. */
export interface IAddAccountHolderRequest {
    'holderKind': string;
    'holderId': string;
    'role'?: string | null;
}
