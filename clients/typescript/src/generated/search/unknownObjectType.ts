export interface IUnknownObjectType {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Search:UnknownObjectType";
    'parameters': {
        objectType: string;
    };
}

export function isUnknownObjectType(arg: any): arg is IUnknownObjectType {
    return arg && arg.errorName === "Search:UnknownObjectType";
}
