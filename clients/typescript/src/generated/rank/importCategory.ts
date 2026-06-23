import { IImportType } from "./importType";

export interface IImportCategory {
    'code': string;
    'name': string;
    'sortOrder'?: number | null;
    'types': Array<IImportType>;
}
