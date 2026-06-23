export interface IMembershipNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Membership:MembershipNotFound";
    'parameters': {
        membershipId: string;
    };
}

export function isMembershipNotFound(arg: any): arg is IMembershipNotFound {
    return arg && arg.errorName === "Membership:MembershipNotFound";
}
