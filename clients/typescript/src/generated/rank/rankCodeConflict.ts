export interface IRankCodeConflict {
    'errorCode': "CONFLICT";
    'errorInstanceId': string;
    'errorName': "Rank:RankCodeConflict";
    'parameters': {
        level: string;
        code: string;
    };
}

export function isRankCodeConflict(arg: any): arg is IRankCodeConflict {
    return arg && arg.errorName === "Rank:RankCodeConflict";
}
