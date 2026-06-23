/** A company appointment a person holds, enriched with the position title + owning company (read-only). */
export interface IPersonCompanyAppointment {
    'id': string;
    'personId': string;
    'positionId': string;
    'positionTitle': string;
    'companyId': string;
    'companyName': string;
    'status': string;
    'effectiveFrom': string;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
