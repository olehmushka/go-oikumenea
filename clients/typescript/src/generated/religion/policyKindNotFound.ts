export interface IPolicyKindNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:PolicyKindNotFound";
    'parameters': {
        policyKindId: string;
    };
}

export function isPolicyKindNotFound(arg: any): arg is IPolicyKindNotFound {
    return arg && arg.errorName === "Religion:PolicyKindNotFound";
}
