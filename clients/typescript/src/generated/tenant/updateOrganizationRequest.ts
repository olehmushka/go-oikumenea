import { Visibility } from "./visibility";

/** Update name/domain/metadata/visibility. Omitted fields are unchanged. `code` is immutable; state is changed via PUT /organizations/{id}/state. */
export interface IUpdateOrganizationRequest {
    'name'?: string | null;
    'domainId'?: string | null;
    'visibility'?: Visibility | null;
    'metadata'?: any | null;
}
