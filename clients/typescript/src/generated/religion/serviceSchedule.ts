/** A per-site recurring service time (a weekly day or an RRULE subset). */
export interface IServiceSchedule {
    'id': string;
    'siteId': string;
    'serviceTypeId': string;
    'serviceTypeCode': string;
    'serviceTypeName': { [key: string]: string };
    /** 0=Sunday … 6=Saturday; null when rrule-driven. */
    'dayOfWeek'?: number | null;
    'rrule'?: string | null;
    /** Local start time as HH:MM. */
    'startTime'?: string | null;
    /** Local end time as HH:MM. */
    'endTime'?: string | null;
    /** IANA zone (e.g. Europe/Kyiv), not a UTC offset. */
    'timezone': string;
    /** Service language (ISO 639-3). */
    'language'?: string | null;
    /** in_person | online | hybrid. */
    'mode': string;
    'meetingUrl'?: string | null;
    'description'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
