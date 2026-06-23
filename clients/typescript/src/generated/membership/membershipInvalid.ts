export interface IMembershipInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Membership:MembershipInvalid";
    'parameters': {
        reason: string;
    };
}

export function isMembershipInvalid(arg: any): arg is IMembershipInvalid {
    return arg && arg.errorName === "Membership:MembershipInvalid";
}
