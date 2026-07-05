import { IAccount } from "./account";
import { IAccountHolder } from "./accountHolder";
import { IAccountHolderList } from "./accountHolderList";
import { IAccountPage } from "./accountPage";
import { IAccountType } from "./accountType";
import { IAccountTypeList } from "./accountTypeList";
import { IAddAccountHolderRequest } from "./addAccountHolderRequest";
import { IAddCardRequest } from "./addCardRequest";
import { ICard } from "./card";
import { ICardList } from "./cardList";
import { ICardNetwork } from "./cardNetwork";
import { ICardNetworkList } from "./cardNetworkList";
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
    listAccounts(institutionId?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IAccountPage>;
    /** Returns the account with the decrypted IBAN for authorized callers. */
    getAccount(accountId: string): Promise<IAccount>;
    updateAccount(accountId: string, request: IUpdateAccountRequest): Promise<IAccount>;
    deleteAccount(accountId: string): Promise<void>;
    listAccountHolders(accountId: string): Promise<IAccountHolderList>;
    addAccountHolder(accountId: string, request: IAddAccountHolderRequest): Promise<IAccountHolder>;
    /** End an active holding (closes effectiveTo); the account and its history remain. */
    endAccountHolding(holderId: string): Promise<IAccountHolder>;
    listCards(accountId: string): Promise<ICardList>;
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

    public listAccounts(institutionId?: string | null, pageSize?: number | null, pageToken?: string | null): Promise<IAccountPage> {
        return this.bridge.call<IAccountPage>(
            "FinanceService",
            "listAccounts",
            "GET",
            "/finance/v1/accounts",
            __undefined,
            __undefined,
            {
                "institutionId": institutionId,
                "pageSize": pageSize,
                "pageToken": pageToken,
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

    public listCards(accountId: string): Promise<ICardList> {
        return this.bridge.call<ICardList>(
            "FinanceService",
            "listCards",
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
