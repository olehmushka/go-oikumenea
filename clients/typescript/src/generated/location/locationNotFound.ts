export interface ILocationNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Location:LocationNotFound";
    'parameters': {
        locationId: string;
    };
}

export function isLocationNotFound(arg: any): arg is ILocationNotFound {
    return arg && arg.errorName === "Location:LocationNotFound";
}
