export interface IChildCreationExcluded {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Religion:ChildCreationExcluded";
    'parameters': {
        unitId: string;
    };
}

export function isChildCreationExcluded(arg: any): arg is IChildCreationExcluded {
    return arg && arg.errorName === "Religion:ChildCreationExcluded";
}
