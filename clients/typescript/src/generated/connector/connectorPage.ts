import { IConnector } from "./connector";

export interface IConnectorPage {
    'connectors': Array<IConnector>;
    'nextPageToken'?: string | null;
}
