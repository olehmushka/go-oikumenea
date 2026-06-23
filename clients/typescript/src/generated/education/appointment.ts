/** A person filling an education position (link__holds_education_position), one-holder, effective-dated. */
export interface IAppointment {
    'id': string;
    'personId': string;
    'positionId': string;
    'status': string;
    'effectiveFrom': string;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
