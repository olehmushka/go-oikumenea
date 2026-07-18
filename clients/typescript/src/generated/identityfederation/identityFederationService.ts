import { IAccount } from "./account";
import { ICreateAccountRequest } from "./createAccountRequest";
import { IExternalIdentity } from "./externalIdentity";
import { IIssuerOption } from "./issuerOption";
import { ILinkIdentityRequest } from "./linkIdentityRequest";
import { IRegisterServicePrincipalRequest } from "./registerServicePrincipalRequest";
import { IServicePrincipal } from "./servicePrincipal";
import { IServicePrincipalPage } from "./servicePrincipalPage";
import { IUpdateServicePrincipalRequest } from "./updateServicePrincipalRequest";
import { IWhoami } from "./whoami";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Optional login accounts + the external identities that federate to them. Account/identity
 * management gates on the `person.*` permissions of the linked person; /whoami is
 * authenticated-only. Token validation itself is middleware, not an endpoint. Writes are audited
 * in-process (D-Audit).
 *
 */
export interface IIdentityFederationService {
    /** Read an account with its linked identities. Gates on `person.read` of the linked person. */
    getAccount(accountId: string): Promise<IAccount>;
    /**
     * List the IdP issuers configured for this instance (public fields only — no secrets), for
     * binding UIs to offer as a choice when linking an external identity. Gates on `person.read`.
     *
     */
    listIssuers(): Promise<Array<IIssuerOption>>;
    /**
     * Read the person's single active account (with its linked identities), or
     * Account:AccountNotFound when the person is roster-only (has no account). Gates on
     * `person.read`. The console uses this to surface a person's login/account state.
     *
     */
    getAccountByPerson(personId: string): Promise<IAccount>;
    /**
     * Create an account for a person, optionally linking its first identity. Gates on
     * `person.update`. Returns Account:AccountConflict if the person already has an active
     * account, Identity:IdentityConflict if the initial (issuer, subject) is already linked.
     *
     */
    createAccount(request: ICreateAccountRequest): Promise<IAccount>;
    /** Disable login on an account (reversible). Gates on `person.update`. */
    disableAccount(accountId: string): Promise<IAccount>;
    /**
     * Link an additional external identity (login point) to an account. Gates on `person.update`.
     * Returns Identity:IdentityConflict when linking is disabled and the account already has an
     * active identity, or when the (issuer, subject) is already linked elsewhere.
     *
     */
    linkIdentity(accountId: string, request: ILinkIdentityRequest): Promise<IExternalIdentity>;
    /** Unlink an external identity from an account (hard removal). Gates on `person.update`. */
    unlinkIdentity(accountId: string, identityId: string): Promise<void>;
    /** Resolve the caller's own PDP context (person + account) from the validated inbound token. */
    whoami(): Promise<IWhoami>;
    /**
     * Register a machine subject (M51 / D-ServiceIdentities). Instance-plane act gated on
     * `service-principal.manage`. Returns ServicePrincipal:ServicePrincipalConflict when the code
     * or the (issuer, subject) is taken — including when that pair is already a person's external
     * identity. Creates no credential: the external IdP owns the client secret.
     *
     */
    registerServicePrincipal(request: IRegisterServicePrincipalRequest): Promise<IServicePrincipal>;
    /** Keyset page over the registry. Gates on `service-principal.read`. */
    listServicePrincipals(pageSize?: number | null, pageToken?: string | null): Promise<IServicePrincipalPage>;
    /** Read one machine subject. Gates on `service-principal.read`. */
    getServicePrincipal(principalId: string): Promise<IServicePrincipal>;
    /**
     * Update the display fields. Gates on `service-principal.manage`. The (issuer, subject) key is
     * immutable — see UpdateServicePrincipalRequest.
     *
     */
    updateServicePrincipal(principalId: string, request: IUpdateServicePrincipalRequest): Promise<IServicePrincipal>;
    /**
     * Reversible kill switch: a disabled principal fails token resolution immediately, while the
     * audit rows naming it stay intact. Gates on `service-principal.manage`.
     *
     */
    disableServicePrincipal(principalId: string): Promise<IServicePrincipal>;
    /** Re-enable a disabled machine subject. Gates on `service-principal.manage`. */
    enableServicePrincipal(principalId: string): Promise<IServicePrincipal>;
}

export class IdentityFederationService implements IIdentityFederationService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Read an account with its linked identities. Gates on `person.read` of the linked person. */
    public getAccount(accountId: string): Promise<IAccount> {
        return this.bridge.call<IAccount>(
            "IdentityFederationService",
            "getAccount",
            "GET",
            "/identity/v1/accounts/{accountId}",
            __undefined,
            __undefined,
            __undefined,
            [
                accountId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * List the IdP issuers configured for this instance (public fields only — no secrets), for
     * binding UIs to offer as a choice when linking an external identity. Gates on `person.read`.
     *
     */
    public listIssuers(): Promise<Array<IIssuerOption>> {
        return this.bridge.call<Array<IIssuerOption>>(
            "IdentityFederationService",
            "listIssuers",
            "GET",
            "/identity/v1/issuers",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Read the person's single active account (with its linked identities), or
     * Account:AccountNotFound when the person is roster-only (has no account). Gates on
     * `person.read`. The console uses this to surface a person's login/account state.
     *
     */
    public getAccountByPerson(personId: string): Promise<IAccount> {
        return this.bridge.call<IAccount>(
            "IdentityFederationService",
            "getAccountByPerson",
            "GET",
            "/identity/v1/persons/{personId}/account",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Create an account for a person, optionally linking its first identity. Gates on
     * `person.update`. Returns Account:AccountConflict if the person already has an active
     * account, Identity:IdentityConflict if the initial (issuer, subject) is already linked.
     *
     */
    public createAccount(request: ICreateAccountRequest): Promise<IAccount> {
        return this.bridge.call<IAccount>(
            "IdentityFederationService",
            "createAccount",
            "POST",
            "/identity/v1/accounts",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Disable login on an account (reversible). Gates on `person.update`. */
    public disableAccount(accountId: string): Promise<IAccount> {
        return this.bridge.call<IAccount>(
            "IdentityFederationService",
            "disableAccount",
            "POST",
            "/identity/v1/accounts/{accountId}/disable",
            __undefined,
            __undefined,
            __undefined,
            [
                accountId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Link an additional external identity (login point) to an account. Gates on `person.update`.
     * Returns Identity:IdentityConflict when linking is disabled and the account already has an
     * active identity, or when the (issuer, subject) is already linked elsewhere.
     *
     */
    public linkIdentity(accountId: string, request: ILinkIdentityRequest): Promise<IExternalIdentity> {
        return this.bridge.call<IExternalIdentity>(
            "IdentityFederationService",
            "linkIdentity",
            "POST",
            "/identity/v1/accounts/{accountId}/identities",
            request,
            __undefined,
            __undefined,
            [
                accountId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Unlink an external identity from an account (hard removal). Gates on `person.update`. */
    public unlinkIdentity(accountId: string, identityId: string): Promise<void> {
        return this.bridge.call<void>(
            "IdentityFederationService",
            "unlinkIdentity",
            "DELETE",
            "/identity/v1/accounts/{accountId}/identities/{identityId}",
            __undefined,
            __undefined,
            __undefined,
            [
                accountId,
                identityId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Resolve the caller's own PDP context (person + account) from the validated inbound token. */
    public whoami(): Promise<IWhoami> {
        return this.bridge.call<IWhoami>(
            "IdentityFederationService",
            "whoami",
            "GET",
            "/identity/v1/whoami",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Register a machine subject (M51 / D-ServiceIdentities). Instance-plane act gated on
     * `service-principal.manage`. Returns ServicePrincipal:ServicePrincipalConflict when the code
     * or the (issuer, subject) is taken — including when that pair is already a person's external
     * identity. Creates no credential: the external IdP owns the client secret.
     *
     */
    public registerServicePrincipal(request: IRegisterServicePrincipalRequest): Promise<IServicePrincipal> {
        return this.bridge.call<IServicePrincipal>(
            "IdentityFederationService",
            "registerServicePrincipal",
            "POST",
            "/identity/v1/service-principals",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Keyset page over the registry. Gates on `service-principal.read`. */
    public listServicePrincipals(pageSize?: number | null, pageToken?: string | null): Promise<IServicePrincipalPage> {
        return this.bridge.call<IServicePrincipalPage>(
            "IdentityFederationService",
            "listServicePrincipals",
            "GET",
            "/identity/v1/service-principals",
            __undefined,
            __undefined,
            {
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Read one machine subject. Gates on `service-principal.read`. */
    public getServicePrincipal(principalId: string): Promise<IServicePrincipal> {
        return this.bridge.call<IServicePrincipal>(
            "IdentityFederationService",
            "getServicePrincipal",
            "GET",
            "/identity/v1/service-principals/{principalId}",
            __undefined,
            __undefined,
            __undefined,
            [
                principalId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Update the display fields. Gates on `service-principal.manage`. The (issuer, subject) key is
     * immutable — see UpdateServicePrincipalRequest.
     *
     */
    public updateServicePrincipal(principalId: string, request: IUpdateServicePrincipalRequest): Promise<IServicePrincipal> {
        return this.bridge.call<IServicePrincipal>(
            "IdentityFederationService",
            "updateServicePrincipal",
            "PUT",
            "/identity/v1/service-principals/{principalId}",
            request,
            __undefined,
            __undefined,
            [
                principalId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Reversible kill switch: a disabled principal fails token resolution immediately, while the
     * audit rows naming it stay intact. Gates on `service-principal.manage`.
     *
     */
    public disableServicePrincipal(principalId: string): Promise<IServicePrincipal> {
        return this.bridge.call<IServicePrincipal>(
            "IdentityFederationService",
            "disableServicePrincipal",
            "POST",
            "/identity/v1/service-principals/{principalId}/disable",
            __undefined,
            __undefined,
            __undefined,
            [
                principalId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Re-enable a disabled machine subject. Gates on `service-principal.manage`. */
    public enableServicePrincipal(principalId: string): Promise<IServicePrincipal> {
        return this.bridge.call<IServicePrincipal>(
            "IdentityFederationService",
            "enableServicePrincipal",
            "POST",
            "/identity/v1/service-principals/{principalId}/enable",
            __undefined,
            __undefined,
            __undefined,
            [
                principalId,
            ],
            __undefined,
            __undefined
        );
    }
}
