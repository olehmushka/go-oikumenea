import { IExplanation } from "./explanation";

/** A PDP decision. explanation is present only when explain was requested. */
export interface IAuthorizeResponse {
    'allow': boolean;
    'explanation'?: IExplanation | null;
}
