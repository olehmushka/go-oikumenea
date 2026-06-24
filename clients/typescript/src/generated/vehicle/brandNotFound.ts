export interface IBrandNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Vehicle:BrandNotFound";
    'parameters': {
        brandId: string;
    };
}

export function isBrandNotFound(arg: any): arg is IBrandNotFound {
    return arg && arg.errorName === "Vehicle:BrandNotFound";
}
