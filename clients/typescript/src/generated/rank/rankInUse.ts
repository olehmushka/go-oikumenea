export interface IRankInUse {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Rank:RankInUse";
    'parameters': {
        reason: string;
    };
}

export function isRankInUse(arg: any): arg is IRankInUse {
    return arg && arg.errorName === "Rank:RankInUse";
}
