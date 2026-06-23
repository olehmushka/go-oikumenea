export interface ICoordinateRequired {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Location:CoordinateRequired";
    'parameters': {
    };
}

export function isCoordinateRequired(arg: any): arg is ICoordinateRequired {
    return arg && arg.errorName === "Location:CoordinateRequired";
}
