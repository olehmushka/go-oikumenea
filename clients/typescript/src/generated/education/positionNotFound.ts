export interface IPositionNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Education:PositionNotFound";
    'parameters': {
        positionId: string;
    };
}

export function isPositionNotFound(arg: any): arg is IPositionNotFound {
    return arg && arg.errorName === "Education:PositionNotFound";
}
