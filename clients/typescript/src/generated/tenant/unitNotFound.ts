export interface IUnitNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Tenant:UnitNotFound";
    'parameters': {
        unitId: string;
    };
}

export function isUnitNotFound(arg: any): arg is IUnitNotFound {
    return arg && arg.errorName === "Tenant:UnitNotFound";
}
