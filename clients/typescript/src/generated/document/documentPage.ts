import { IDocument } from "./document";

/** A page of documents plus the opaque token for the next page (empty when exhausted). */
export interface IDocumentPage {
    'documents': Array<IDocument>;
    'nextPageToken'?: string | null;
}
