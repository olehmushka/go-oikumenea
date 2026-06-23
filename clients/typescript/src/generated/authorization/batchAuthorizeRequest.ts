import { IAuthorizeQuery } from "./authorizeQuery";

/** Several questions for one subject; the subject's authority state is fetched once. */
export interface IBatchAuthorizeRequest {
    'subjectPersonId': string;
    'queries': Array<IAuthorizeQuery>;
    'explain'?: boolean | null;
}
