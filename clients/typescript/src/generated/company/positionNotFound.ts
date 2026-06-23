export interface IPositionNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Company:PositionNotFound";
    'parameters': {
        positionId: string;
    };
}

export function isPositionNotFound(arg: any): arg is IPositionNotFound {
    return arg && arg.errorName === "Company:PositionNotFound";
}
