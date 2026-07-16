export interface IUnknownObjectType {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Links:UnknownObjectType";
    'parameters': {
        rid: string;
    };
}

export function isUnknownObjectType(arg: any): arg is IUnknownObjectType {
    return arg && arg.errorName === "Links:UnknownObjectType";
}
