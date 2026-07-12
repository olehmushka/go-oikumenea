import { IPublication } from "./publication";

export interface IPublicationPage {
    'publications': Array<IPublication>;
    'nextPageToken'?: string | null;
}
