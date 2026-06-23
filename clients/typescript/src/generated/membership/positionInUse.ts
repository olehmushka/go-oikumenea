export interface IPositionInUse {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Position:PositionInUse";
    'parameters': {
        positionId: string;
    };
}

export function isPositionInUse(arg: any): arg is IPositionInUse {
    return arg && arg.errorName === "Position:PositionInUse";
}
