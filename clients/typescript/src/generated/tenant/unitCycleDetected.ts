export interface IUnitCycleDetected {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Tenant:UnitCycleDetected";
    'parameters': {
        graph: string;
        parentId: string;
        childId: string;
    };
}

export function isUnitCycleDetected(arg: any): arg is IUnitCycleDetected {
    return arg && arg.errorName === "Tenant:UnitCycleDetected";
}
