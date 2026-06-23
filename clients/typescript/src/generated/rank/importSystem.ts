import { IImportCategory } from "./importCategory";

export interface IImportSystem {
    'code': string;
    'name': string;
    'country'?: string | null;
    'sortOrder'?: number | null;
    'categories': Array<IImportCategory>;
}
