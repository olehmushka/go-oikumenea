export interface IMembershipConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Membership:MembershipConflict";
    'parameters': {
        reason: string;
    };
}

export function isMembershipConflict(arg: any): arg is IMembershipConflict {
    return arg && arg.errorName === "Membership:MembershipConflict";
}
