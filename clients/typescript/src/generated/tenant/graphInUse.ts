export interface IGraphInUse {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Tenant:GraphInUse";
    'parameters': {
        graph: string;
    };
}

export function isGraphInUse(arg: any): arg is IGraphInUse {
    return arg && arg.errorName === "Tenant:GraphInUse";
}
