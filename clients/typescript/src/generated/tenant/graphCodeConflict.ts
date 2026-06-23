export interface IGraphCodeConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Tenant:GraphCodeConflict";
    'parameters': {
        code: string;
    };
}

export function isGraphCodeConflict(arg: any): arg is IGraphCodeConflict {
    return arg && arg.errorName === "Tenant:GraphCodeConflict";
}
