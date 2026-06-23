import { IAppointment } from "./appointment";

/**
 * An institution/unit-owned billet (mirrors membership Position) — exists while vacant; carries
 * no authority. The current holder is populated on get/list (null when vacant).
 *
 */
export interface IEducationPosition {
    'id': string;
    'institutionId': string;
    'unitId'?: string | null;
    'code': string;
    'title': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
    'holder'?: IAppointment | null;
    'createdAt': string;
    'updatedAt': string;
}
