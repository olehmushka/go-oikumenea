import { IAuditEntry } from "./auditEntry";

/** A page of audit entries plus the opaque token for the next page (empty when exhausted). */
export interface IAuditEntryPage {
    'entries': Array<IAuditEntry>;
    'nextPageToken'?: string | null;
}
