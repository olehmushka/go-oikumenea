export interface IRegionInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Vehicle:RegionInvalid";
    'parameters': {
        subdivisionId: string;
    };
}

export function isRegionInvalid(arg: any): arg is IRegionInvalid {
    return arg && arg.errorName === "Vehicle:RegionInvalid";
}
