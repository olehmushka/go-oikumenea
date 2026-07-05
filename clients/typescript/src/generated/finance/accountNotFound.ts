export interface IAccountNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Finance:AccountNotFound";
    'parameters': {
        accountId: string;
    };
}

export function isAccountNotFound(arg: any): arg is IAccountNotFound {
    return arg && arg.errorName === "Finance:AccountNotFound";
}
