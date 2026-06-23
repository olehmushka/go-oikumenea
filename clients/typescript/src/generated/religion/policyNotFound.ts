export interface IPolicyNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:PolicyNotFound";
    'parameters': {
        policyId: string;
    };
}

export function isPolicyNotFound(arg: any): arg is IPolicyNotFound {
    return arg && arg.errorName === "Religion:PolicyNotFound";
}
