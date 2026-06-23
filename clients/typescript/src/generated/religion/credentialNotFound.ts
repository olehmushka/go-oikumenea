export interface ICredentialNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:CredentialNotFound";
    'parameters': {
        credentialId: string;
    };
}

export function isCredentialNotFound(arg: any): arg is ICredentialNotFound {
    return arg && arg.errorName === "Religion:CredentialNotFound";
}
