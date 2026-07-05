import { IPersonAccount } from "./personAccount";

/** A person's held accounts (newest first). */
export interface IPersonAccounts {
    'accounts': Array<IPersonAccount>;
}
