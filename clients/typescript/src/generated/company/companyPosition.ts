import { IAppointment } from "./appointment";

/**
 * A company-owned billet (CEO/director/employee line; mirrors membership Position) — exists while
 * vacant; carries no authority. The current holder is populated on get/list (null when vacant).
 *
 */
export interface ICompanyPosition {
    'id': string;
    'companyId': string;
    'code': string;
    'title': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
    'holder'?: IAppointment | null;
    'createdAt': string;
    'updatedAt': string;
}
