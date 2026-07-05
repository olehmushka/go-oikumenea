import { IAccount } from "./account";

export interface IAccountPage {
    'accounts': Array<IAccount>;
    'nextPageToken'?: string | null;
}
