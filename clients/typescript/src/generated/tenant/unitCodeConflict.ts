export interface IUnitCodeConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Tenant:UnitCodeConflict";
    'parameters': {
        code: string;
    };
}

export function isUnitCodeConflict(arg: any): arg is IUnitCodeConflict {
    return arg && arg.errorName === "Tenant:UnitCodeConflict";
}
