export interface IGraphNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Tenant:GraphNotFound";
    'parameters': {
        graph: string;
    };
}

export function isGraphNotFound(arg: any): arg is IGraphNotFound {
    return arg && arg.errorName === "Tenant:GraphNotFound";
}
