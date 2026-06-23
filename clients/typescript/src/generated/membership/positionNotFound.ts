export interface IPositionNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Position:PositionNotFound";
    'parameters': {
        positionId: string;
    };
}

export function isPositionNotFound(arg: any): arg is IPositionNotFound {
    return arg && arg.errorName === "Position:PositionNotFound";
}
