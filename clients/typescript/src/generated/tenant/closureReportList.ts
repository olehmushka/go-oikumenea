import { IClosureReport } from "./closureReport";

/** Per-graph closure reports (one per graph processed). */
export interface IClosureReportList {
    'reports': Array<IClosureReport>;
}
