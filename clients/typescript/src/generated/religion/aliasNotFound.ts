export interface IAliasNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:AliasNotFound";
    'parameters': {
        aliasId: string;
    };
}

export function isAliasNotFound(arg: any): arg is IAliasNotFound {
    return arg && arg.errorName === "Religion:AliasNotFound";
}
