import { ICompany } from "./company";

export interface ICompanyPage {
    'companies': Array<ICompany>;
    'nextPageToken'?: string | null;
}
