export interface IPositionConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Position:PositionConflict";
    'parameters': {
        reason: string;
    };
}

export function isPositionConflict(arg: any): arg is IPositionConflict {
    return arg && arg.errorName === "Position:PositionConflict";
}
