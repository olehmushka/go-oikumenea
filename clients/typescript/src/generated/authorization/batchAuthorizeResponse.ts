import { IAuthorizeResponse } from "./authorizeResponse";

export interface IBatchAuthorizeResponse {
    'decisions': Array<IAuthorizeResponse>;
}
