export interface IMembershipLifecycleConflict {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Membership:MembershipLifecycleConflict";
    'parameters': {
        reason: string;
    };
}

export function isMembershipLifecycleConflict(arg: any): arg is IMembershipLifecycleConflict {
    return arg && arg.errorName === "Membership:MembershipLifecycleConflict";
}
