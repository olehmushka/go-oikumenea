import { ISelfConnector } from "./selfConnector";
import { ISelfSource } from "./selfSource";

export interface IWiringSelf {
    'connector': ISelfConnector;
    'sources': Array<ISelfSource>;
}
