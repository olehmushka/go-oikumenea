export interface IDemoError {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Platform:DemoError";
    'parameters': {
        reason: string;
    };
}

export function isDemoError(arg: any): arg is IDemoError {
    return arg && arg.errorName === "Platform:DemoError";
}
