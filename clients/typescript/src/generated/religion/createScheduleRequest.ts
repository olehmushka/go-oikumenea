/** A recurring service time — supply dayOfWeek (0=Sunday…6=Saturday) OR an rrule. meetingUrl is required when mode is online/hybrid. */
export interface ICreateScheduleRequest {
    'serviceTypeId': string;
    'dayOfWeek'?: number | null;
    'rrule'?: string | null;
    /** Local start time as HH:MM. */
    'startTime'?: string | null;
    /** Local end time as HH:MM. */
    'endTime'?: string | null;
    'timezone': string;
    'language'?: string | null;
    /** in_person | online | hybrid (default in_person). */
    'mode'?: string | null;
    'meetingUrl'?: string | null;
    'description'?: string | null;
}
