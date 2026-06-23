export interface IPositionAlreadyFilled {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Education:PositionAlreadyFilled";
    'parameters': {
        positionId: string;
    };
}

export function isPositionAlreadyFilled(arg: any): arg is IPositionAlreadyFilled {
    return arg && arg.errorName === "Education:PositionAlreadyFilled";
}
