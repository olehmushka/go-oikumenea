import { IServicePrincipal } from "./servicePrincipal";

export interface IServicePrincipalPage {
    'principals': Array<IServicePrincipal>;
    'nextPageToken'?: string | null;
}
