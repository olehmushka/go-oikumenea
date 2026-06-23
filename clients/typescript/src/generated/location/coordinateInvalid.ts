export interface ICoordinateInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Location:CoordinateInvalid";
    'parameters': {
    };
}

export function isCoordinateInvalid(arg: any): arg is ICoordinateInvalid {
    return arg && arg.errorName === "Location:CoordinateInvalid";
}
