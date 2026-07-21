import { ISourceDeclaration } from "./sourceDeclaration";

/**
 * A connector's idempotent self-description. Carries NO principalId: the core binds the row to
 * the calling principal, so a connector cannot register as another. Re-registering updates the
 * row and REPLACES the declared source set (sources absent from the payload are retired), which
 * makes boot-time registration converge without an operator reconciling drift by hand.
 *
 */
export interface IRegisterConnectorRequest {
    'code': string;
    'name': string;
    'description'?: string | null;
    'sources': Array<ISourceDeclaration>;
}
