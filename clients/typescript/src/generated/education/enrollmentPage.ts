import { IEnrollment } from "./enrollment";

export interface IEnrollmentPage {
    'enrollments': Array<IEnrollment>;
    'nextPageToken'?: string | null;
}
