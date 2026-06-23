/** An appointment a person holds, enriched with the position's title and owning institution (read-only person view). */
export interface IPersonAppointment {
    'id': string;
    'personId': string;
    'positionId': string;
    'positionTitle': string;
    'institutionId': string;
    'institutionName': string;
    'status': string;
    'effectiveFrom': string;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
