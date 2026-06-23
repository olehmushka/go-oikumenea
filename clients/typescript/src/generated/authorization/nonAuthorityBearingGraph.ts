export interface INonAuthorityBearingGraph {
    'errorCode': "FAILED_PRECONDITION";
    'errorInstanceId': string;
    'errorName': "Authorization:NonAuthorityBearingGraph";
    'parameters': {
        graph: string;
    };
}

export function isNonAuthorityBearingGraph(arg: any): arg is INonAuthorityBearingGraph {
    return arg && arg.errorName === "Authorization:NonAuthorityBearingGraph";
}
