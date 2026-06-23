export interface IRankInvalid {
    'errorCode': "INVALID_ARGUMENT";
    'errorInstanceId': string;
    'errorName': "Rank:RankInvalid";
    'parameters': {
        reason: string;
    };
}

export function isRankInvalid(arg: any): arg is IRankInvalid {
    return arg && arg.errorName === "Rank:RankInvalid";
}
