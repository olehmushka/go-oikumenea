export interface ISourceNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Connector:SourceNotFound";
    'parameters': {
        sourceCode: string;
    };
}

export function isSourceNotFound(arg: any): arg is ISourceNotFound {
    return arg && arg.errorName === "Connector:SourceNotFound";
}
