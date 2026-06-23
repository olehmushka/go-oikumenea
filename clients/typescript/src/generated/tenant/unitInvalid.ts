export interface IUnitInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Tenant:UnitInvalid";
    'parameters': {
        reason: string;
    };
}

export function isUnitInvalid(arg: any): arg is IUnitInvalid {
    return arg && arg.errorName === "Tenant:UnitInvalid";
}
