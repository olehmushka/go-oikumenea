export interface ITransitionInvalid {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Tenant:TransitionInvalid";
    'parameters': {
        reason: string;
    };
}

export function isTransitionInvalid(arg: any): arg is ITransitionInvalid {
    return arg && arg.errorName === "Tenant:TransitionInvalid";
}
