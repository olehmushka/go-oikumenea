export interface ISelfEscalation {
    'errorCode': "PERMISSION_DENIED";
    'errorInstanceId': string;
    'errorName': "Authorization:SelfEscalation";
    'parameters': {
        reason: string;
    };
}

export function isSelfEscalation(arg: any): arg is ISelfEscalation {
    return arg && arg.errorName === "Authorization:SelfEscalation";
}
