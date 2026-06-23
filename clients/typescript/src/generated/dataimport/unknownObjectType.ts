export interface IUnknownObjectType {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "DataImport:UnknownObjectType";
    'parameters': {
        objectType: string;
    };
}

export function isUnknownObjectType(arg: any): arg is IUnknownObjectType {
    return arg && arg.errorName === "DataImport:UnknownObjectType";
}
