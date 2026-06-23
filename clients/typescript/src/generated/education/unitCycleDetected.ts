export interface IUnitCycleDetected {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Education:UnitCycleDetected";
    'parameters': {
        unitId: string;
    };
}

export function isUnitCycleDetected(arg: any): arg is IUnitCycleDetected {
    return arg && arg.errorName === "Education:UnitCycleDetected";
}
