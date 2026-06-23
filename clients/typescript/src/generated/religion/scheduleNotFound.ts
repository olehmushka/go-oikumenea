export interface IScheduleNotFound {
    'errorCode': "NOT_FOUND";
    'errorInstanceId': string;
    'errorName': "Religion:ScheduleNotFound";
    'parameters': {
        scheduleId: string;
    };
}

export function isScheduleNotFound(arg: any): arg is IScheduleNotFound {
    return arg && arg.errorName === "Religion:ScheduleNotFound";
}
