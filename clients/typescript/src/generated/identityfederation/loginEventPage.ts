import { ILoginEvent } from "./loginEvent";

export interface ILoginEventPage {
    'events': Array<ILoginEvent>;
    'nextPageToken'?: string | null;
}
