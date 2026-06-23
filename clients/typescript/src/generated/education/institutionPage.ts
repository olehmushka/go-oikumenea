import { IInstitution } from "./institution";

export interface IInstitutionPage {
    'institutions': Array<IInstitution>;
    'nextPageToken'?: string | null;
}
