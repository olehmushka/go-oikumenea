export interface IDormitoryStayNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Education:DormitoryStayNotFound";
    'parameters': {
        stayId: string;
    };
}

export function isDormitoryStayNotFound(arg: any): arg is IDormitoryStayNotFound {
    return arg && arg.errorName === "Education:DormitoryStayNotFound";
}
