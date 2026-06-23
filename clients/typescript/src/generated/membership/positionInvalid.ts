export interface IPositionInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Position:PositionInvalid";
    'parameters': {
        reason: string;
    };
}

export function isPositionInvalid(arg: any): arg is IPositionInvalid {
    return arg && arg.errorName === "Position:PositionInvalid";
}
