export interface IGraphProtected {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Tenant:GraphProtected";
    'parameters': {
        reason: string;
    };
}

export function isGraphProtected(arg: any): arg is IGraphProtected {
    return arg && arg.errorName === "Tenant:GraphProtected";
}
