export interface ILocationInUse {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Location:LocationInUse";
    'parameters': {
        locationId: string;
    };
}

export function isLocationInUse(arg: any): arg is ILocationInUse {
    return arg && arg.errorName === "Location:LocationInUse";
}
