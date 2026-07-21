import { ISyncRun } from "./syncRun";

export interface ISyncRunPage {
    'runs': Array<ISyncRun>;
    'nextPageToken'?: string | null;
}
