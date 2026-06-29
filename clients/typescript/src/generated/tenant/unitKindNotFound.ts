export interface IUnitKindNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Tenant:UnitKindNotFound";
    'parameters': {
        unitKindId: string;
    };
}

export function isUnitKindNotFound(arg: any): arg is IUnitKindNotFound {
    return arg && arg.errorName === "Tenant:UnitKindNotFound";
}
