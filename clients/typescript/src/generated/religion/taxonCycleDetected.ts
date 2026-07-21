export interface ITaxonCycleDetected {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Religion:TaxonCycleDetected";
    'parameters': {
        taxonId: string;
    };
}

export function isTaxonCycleDetected(arg: any): arg is ITaxonCycleDetected {
    return arg && arg.errorName === "Religion:TaxonCycleDetected";
}
