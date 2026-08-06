import { IAccount } from "./account";
import { IAccountHolder } from "./accountHolder";
import { IAccountHolderList } from "./accountHolderList";
import { IAccountPage } from "./accountPage";
import { IAccountStats } from "./accountStats";
import { IAccountType } from "./accountType";
import { IAccountTypeList } from "./accountTypeList";
import { IAddAccountHolderRequest } from "./addAccountHolderRequest";
import { IAddCardRequest } from "./addCardRequest";
import { ICard } from "./card";
import { ICardList } from "./cardList";
import { ICardNetwork } from "./cardNetwork";
import { ICardNetworkList } from "./cardNetworkList";
import { ICardPage } from "./cardPage";
import { ICardStats } from "./cardStats";
import { ICreateAccountRequest } from "./createAccountRequest";
import { IPersonAccounts } from "./personAccounts";
import { IUpdateAccountRequest } from "./updateAccountRequest";
import { IUpdateCardRequest } from "./updateCardRequest";
import { IUpsertAccountTypeRequest } from "./upsertAccountTypeRequest";
import { IUpsertCardNetworkRequest } from "./upsertCardNetworkRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Bank accounts & payment cards: account-type / card-network catalogs, accounts (encrypted IBAN),
 * the polymorphic holder link, and cards (encrypted PAN + clear BIN/last-4). Reads gate on
 * `finance.read`; account/card/holder writes on `finance.manage`; catalog-entry writes on
 * `finance.catalog.manage`. Writes are audited in-process (D-Audit).
 *
 */
export interface IFinanceService {
    listAccountTypes(): Promise<IAccountTypeList>;
    upsertAccountType(request: IUpsertAccountTypeRequest): Promise<IAccountType>;
    listCardNetworks(): Promise<ICardNetworkList>;
    upsertCardNetwork(request: IUpsertCardNetworkRequest): Promise<ICardNetwork>;
    createAccount(request: ICreateAccountRequest): Promise<IAccount>;
    /**
     * List accounts, token-paginated, narrowed by any combination of the facet filters below
     * (M58 / D-ObjectFacets). Every filter here is also a distribution on `accountStats`. The
     * IBAN is never listed. Gated by `finance.read`.
     *
     */
    listAccounts(institutionId?: string | null, currency?: string | null, accountTypeId?: string | null, status?: string | null, holderKind?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IAccountPage>;
    /**
     * Facet distributions over the account registry — the dashboard half of the facet vocabulary
     * (M58 / D-ObjectFacets). Takes exactly the filter args `listAccounts` takes (minus paging)
     * plus an optional `facets` CSV, so a dashboard and a list are two renderings of ONE request
     * state and a chart segment is a link to the same URL with one more filter applied.
     *
     * `totalCount` equals the number of rows exhaustively paging `listAccounts` with these same
     * filters would return. One round-trip serves the whole dashboard.
     *
     * ONE aggregate arm, with no subject and no scoped twin — for `externalOrgStats`' reason, not
     * the audit ledger's. `finance_accounts` carries no row-level security and no unit reach:
     * `finance.read` held anywhere is the whole visibility decision, so there is nothing for a
     * second arm to narrow. (Person-held rows are additionally holder-scoped on the PERSON views;
     * that is a different endpoint, and this registry-level listing is not one of them.)
     *
     * The path is `/stats/accounts` rather than `/accounts/stats` because the server's router
     * rejects a literal path segment that is a sibling of `{accountId}`.
     *
     */
    accountStats(facets?: string | null, institutionId?: string | null, currency?: string | null, accountTypeId?: string | null, status?: string | null, holderKind?: string | null): Promise<IAccountStats>;
    /** Returns the account with the decrypted IBAN for authorized callers. */
    getAccount(accountId: string): Promise<IAccount>;
    updateAccount(accountId: string, request: IUpdateAccountRequest): Promise<IAccount>;
    deleteAccount(accountId: string): Promise<void>;
    listAccountHolders(accountId: string): Promise<IAccountHolderList>;
    addAccountHolder(accountId: string, request: IAddAccountHolderRequest): Promise<IAccountHolder>;
    /** End an active holding (closes effectiveTo); the account and its history remain. */
    endAccountHolding(holderId: string): Promise<IAccountHolder>;
    /**
     * The cards on ONE account. Named for its scope, beside `listAccountHolders` — M58 gave the
     * plain `listCards` to the instance-wide registry below, which is the collection every other
     * faceted object type's list endpoint is named for. The HTTP path is unchanged.
     *
     */
    listAccountCards(accountId: string): Promise<ICardList>;
    /**
     * The instance-wide card registry, token-paginated and narrowed by the facet filters below
     * (M58 / D-ObjectFacets) — the collection-level list `cardStats` draws its dashboard over.
     *
     * METADATA ONLY. `bin`, `lastFour`, network, type, status and expiry are clear columns and are
     * returned; the PAN is envelope-encrypted at rest and is decrypted only by `getCard`, for an
     * authorized caller, one card at a time (PCI-DSS Req 3; D-DataScope CDE scope). Browsing the
     * registry is gated by the same `finance.read` that already gates `listAccounts` and
     * `listAccountCards` — this endpoint widens the SCOPE of a read the code already permits, and
     * discloses no field those endpoints did not already return.
     *
     */
    listCards(networkId?: string | null, cardType?: string | null, status?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ICardPage>;
    /**
     * Facet distributions over the card registry — the dashboard half of the facet vocabulary
     * (M58 / D-ObjectFacets). Takes exactly the filter args `listCards` takes (minus paging) plus
     * an optional `facets` CSV.
     *
     * `totalCount` equals the number of rows exhaustively paging `listCards` with these same
     * filters would return.
     *
     * ONE aggregate arm, for the same reason `accountStats` has one: no row-level security, no
     * unit reach, `finance.read` held anywhere is the whole visibility decision.
     *
     * The path is `/stats/cards` rather than `/cards/stats` because the server's router rejects a
     * literal path segment that is a sibling of `{cardId}`.
     *
     */
    cardStats(facets?: string | null, networkId?: string | null, cardType?: string | null, status?: string | null): Promise<ICardStats>;
    addCard(accountId: string, request: IAddCardRequest): Promise<ICard>;
    /** Returns the card with the decrypted PAN for authorized callers. */
    getCard(cardId: string): Promise<ICard>;
    updateCard(cardId: string, request: IUpdateCardRequest): Promise<ICard>;
    deleteCard(cardId: string): Promise<void>;
    /** Read-only view of the accounts a person holds. */
    listPersonAccounts(personId: string): Promise<IPersonAccounts>;
}

export class FinanceService implements IFinanceService {
    constructor(private bridge: IHttpApiBridge) {
    }

    public listAccountTypes(): Promise<IAccountTypeList> {
        return this.bridge.call<IAccountTypeList>(
            "FinanceService",
            "listAccountTypes",
            "GET",
            "/finance/v1/account-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertAccountType(request: IUpsertAccountTypeRequest): Promise<IAccountType> {
        return this.bridge.call<IAccountType>(
            "FinanceService",
            "upsertAccountType",
            "PUT",
            "/finance/v1/account-types",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public listCardNetworks(): Promise<ICardNetworkList> {
        return this.bridge.call<ICardNetworkList>(
            "FinanceService",
            "listCardNetworks",
            "GET",
            "/finance/v1/card-networks",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public upsertCardNetwork(request: IUpsertCardNetworkRequest): Promise<ICardNetwork> {
        return this.bridge.call<ICardNetwork>(
            "FinanceService",
            "upsertCardNetwork",
            "PUT",
            "/finance/v1/card-networks",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    public createAccount(request: ICreateAccountRequest): Promise<IAccount> {
        return this.bridge.call<IAccount>(
            "FinanceService",
            "createAccount",
            "POST",
            "/finance/v1/accounts",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * List accounts, token-paginated, narrowed by any combination of the facet filters below
     * (M58 / D-ObjectFacets). Every filter here is also a distribution on `accountStats`. The
     * IBAN is never listed. Gated by `finance.read`.
     *
     */
    public listAccounts(institutionId?: string | null, currency?: string | null, accountTypeId?: string | null, status?: string | null, holderKind?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IAccountPage> {
        return this.bridge.call<IAccountPage>(
            "FinanceService",
            "listAccounts",
            "GET",
            "/finance/v1/accounts",
            __undefined,
            __undefined,
            {
                "institutionId": institutionId,
                "currency": currency,
                "accountTypeId": accountTypeId,
                "status": status,
                "holderKind": holderKind,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Facet distributions over the account registry — the dashboard half of the facet vocabulary
     * (M58 / D-ObjectFacets). Takes exactly the filter args `listAccounts` takes (minus paging)
     * plus an optional `facets` CSV, so a dashboard and a list are two renderings of ONE request
     * state and a chart segment is a link to the same URL with one more filter applied.
     *
     * `totalCount` equals the number of rows exhaustively paging `listAccounts` with these same
     * filters would return. One round-trip serves the whole dashboard.
     *
     * ONE aggregate arm, with no subject and no scoped twin — for `externalOrgStats`' reason, not
     * the audit ledger's. `finance_accounts` carries no row-level security and no unit reach:
     * `finance.read` held anywhere is the whole visibility decision, so there is nothing for a
     * second arm to narrow. (Person-held rows are additionally holder-scoped on the PERSON views;
     * that is a different endpoint, and this registry-level listing is not one of them.)
     *
     * The path is `/stats/accounts` rather than `/accounts/stats` because the server's router
     * rejects a literal path segment that is a sibling of `{accountId}`.
     *
     */
    public accountStats(facets?: string | null, institutionId?: string | null, currency?: string | null, accountTypeId?: string | null, status?: string | null, holderKind?: string | null): Promise<IAccountStats> {
        return this.bridge.call<IAccountStats>(
            "FinanceService",
            "accountStats",
            "GET",
            "/finance/v1/stats/accounts",
            __undefined,
            __undefined,
            {
                "facets": facets,
                "institutionId": institutionId,
                "currency": currency,
                "accountTypeId": accountTypeId,
                "status": status,
                "holderKind": holderKind,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Returns the account with the decrypted IBAN for authorized callers. */
    public getAccount(accountId: string): Promise<IAccount> {
        return this.bridge.call<IAccount>(
            "FinanceService",
            "getAccount",
            "GET",
            "/finance/v1/accounts/{accountId}",
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

    public updateAccount(accountId: string, request: IUpdateAccountRequest): Promise<IAccount> {
        return this.bridge.call<IAccount>(
            "FinanceService",
            "updateAccount",
            "PUT",
            "/finance/v1/accounts/{accountId}",
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

    public deleteAccount(accountId: string): Promise<void> {
        return this.bridge.call<void>(
            "FinanceService",
            "deleteAccount",
            "DELETE",
            "/finance/v1/accounts/{accountId}",
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

    public listAccountHolders(accountId: string): Promise<IAccountHolderList> {
        return this.bridge.call<IAccountHolderList>(
            "FinanceService",
            "listAccountHolders",
            "GET",
            "/finance/v1/accounts/{accountId}/holders",
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

    public addAccountHolder(accountId: string, request: IAddAccountHolderRequest): Promise<IAccountHolder> {
        return this.bridge.call<IAccountHolder>(
            "FinanceService",
            "addAccountHolder",
            "POST",
            "/finance/v1/accounts/{accountId}/holders",
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

    /** End an active holding (closes effectiveTo); the account and its history remain. */
    public endAccountHolding(holderId: string): Promise<IAccountHolder> {
        return this.bridge.call<IAccountHolder>(
            "FinanceService",
            "endAccountHolding",
            "POST",
            "/finance/v1/holders/{holderId}/end",
            __undefined,
            __undefined,
            __undefined,
            [
                holderId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * The cards on ONE account. Named for its scope, beside `listAccountHolders` — M58 gave the
     * plain `listCards` to the instance-wide registry below, which is the collection every other
     * faceted object type's list endpoint is named for. The HTTP path is unchanged.
     *
     */
    public listAccountCards(accountId: string): Promise<ICardList> {
        return this.bridge.call<ICardList>(
            "FinanceService",
            "listAccountCards",
            "GET",
            "/finance/v1/accounts/{accountId}/cards",
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
     * The instance-wide card registry, token-paginated and narrowed by the facet filters below
     * (M58 / D-ObjectFacets) — the collection-level list `cardStats` draws its dashboard over.
     *
     * METADATA ONLY. `bin`, `lastFour`, network, type, status and expiry are clear columns and are
     * returned; the PAN is envelope-encrypted at rest and is decrypted only by `getCard`, for an
     * authorized caller, one card at a time (PCI-DSS Req 3; D-DataScope CDE scope). Browsing the
     * registry is gated by the same `finance.read` that already gates `listAccounts` and
     * `listAccountCards` — this endpoint widens the SCOPE of a read the code already permits, and
     * discloses no field those endpoints did not already return.
     *
     */
    public listCards(networkId?: string | null, cardType?: string | null, status?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<ICardPage> {
        return this.bridge.call<ICardPage>(
            "FinanceService",
            "listCards",
            "GET",
            "/finance/v1/cards",
            __undefined,
            __undefined,
            {
                "networkId": networkId,
                "cardType": cardType,
                "status": status,
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Facet distributions over the card registry — the dashboard half of the facet vocabulary
     * (M58 / D-ObjectFacets). Takes exactly the filter args `listCards` takes (minus paging) plus
     * an optional `facets` CSV.
     *
     * `totalCount` equals the number of rows exhaustively paging `listCards` with these same
     * filters would return.
     *
     * ONE aggregate arm, for the same reason `accountStats` has one: no row-level security, no
     * unit reach, `finance.read` held anywhere is the whole visibility decision.
     *
     * The path is `/stats/cards` rather than `/cards/stats` because the server's router rejects a
     * literal path segment that is a sibling of `{cardId}`.
     *
     */
    public cardStats(facets?: string | null, networkId?: string | null, cardType?: string | null, status?: string | null): Promise<ICardStats> {
        return this.bridge.call<ICardStats>(
            "FinanceService",
            "cardStats",
            "GET",
            "/finance/v1/stats/cards",
            __undefined,
            __undefined,
            {
                "facets": facets,
                "networkId": networkId,
                "cardType": cardType,
                "status": status,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    public addCard(accountId: string, request: IAddCardRequest): Promise<ICard> {
        return this.bridge.call<ICard>(
            "FinanceService",
            "addCard",
            "POST",
            "/finance/v1/accounts/{accountId}/cards",
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

    /** Returns the card with the decrypted PAN for authorized callers. */
    public getCard(cardId: string): Promise<ICard> {
        return this.bridge.call<ICard>(
            "FinanceService",
            "getCard",
            "GET",
            "/finance/v1/cards/{cardId}",
            __undefined,
            __undefined,
            __undefined,
            [
                cardId,
            ],
            __undefined,
            __undefined
        );
    }

    public updateCard(cardId: string, request: IUpdateCardRequest): Promise<ICard> {
        return this.bridge.call<ICard>(
            "FinanceService",
            "updateCard",
            "PUT",
            "/finance/v1/cards/{cardId}",
            request,
            __undefined,
            __undefined,
            [
                cardId,
            ],
            __undefined,
            __undefined
        );
    }

    public deleteCard(cardId: string): Promise<void> {
        return this.bridge.call<void>(
            "FinanceService",
            "deleteCard",
            "DELETE",
            "/finance/v1/cards/{cardId}",
            __undefined,
            __undefined,
            __undefined,
            [
                cardId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Read-only view of the accounts a person holds. */
    public listPersonAccounts(personId: string): Promise<IPersonAccounts> {
        return this.bridge.call<IPersonAccounts>(
            "FinanceService",
            "listPersonAccounts",
            "GET",
            "/finance/v1/persons/{personId}/accounts",
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
}
