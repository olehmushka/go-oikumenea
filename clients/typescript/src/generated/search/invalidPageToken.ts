export interface IInvalidPageToken {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Search:InvalidPageToken";
    'parameters': {
    };
}

export function isInvalidPageToken(arg: any): arg is IInvalidPageToken {
    return arg && arg.errorName === "Search:InvalidPageToken";
}
