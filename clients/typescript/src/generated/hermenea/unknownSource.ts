export interface IUnknownSource {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Hermenea:UnknownSource";
    'parameters': {
        source: string;
    };
}

export function isUnknownSource(arg: any): arg is IUnknownSource {
    return arg && arg.errorName === "Hermenea:UnknownSource";
}
