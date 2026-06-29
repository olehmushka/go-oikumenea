export interface IUnitKindCodeConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Tenant:UnitKindCodeConflict";
    'parameters': {
        domainId: string;
        code: string;
    };
}

export function isUnitKindCodeConflict(arg: any): arg is IUnitKindCodeConflict {
    return arg && arg.errorName === "Tenant:UnitKindCodeConflict";
}
