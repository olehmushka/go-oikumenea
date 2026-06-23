import { ICompanyPosition } from "./companyPosition";

export interface ICompanyPositionPage {
    'positions': Array<ICompanyPosition>;
    'nextPageToken'?: string | null;
}
