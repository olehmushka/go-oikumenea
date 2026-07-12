import { IScholarship } from "./scholarship";

export interface IScholarshipPage {
    'scholarships': Array<IScholarship>;
    'nextPageToken'?: string | null;
}
