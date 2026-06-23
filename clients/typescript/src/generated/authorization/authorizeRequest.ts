/** Ask the PDP whether subject may perform action on unit. unitId is omitted for instance-scope actions. */
export interface IAuthorizeRequest {
    'subjectPersonId': string;
    'action': string;
    'unitId'?: string | null;
    /** When true, the response carries an explanation (DS-16). Default false. */
    'explain'?: boolean | null;
}
