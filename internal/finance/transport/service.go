// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package transport implements the generated financeapi.FinanceService (D-Finance, M44). It PEP-gates
// each op (finance entities are authoritative first-party directory data, not tenant-unit scoped, so
// reads/writes are satisfied anywhere; person-held rows are additionally holder-scoped), assembles
// translatable catalog labels (account-type / card-network names) as locale->text maps via the
// localization service, resolves best-effort display labels (bank org name, account-type/network names),
// and maps domain sentinels to the Conjure Finance:* SerializableErrors. The encrypted IBAN/PAN are
// decrypted by the application service only on the single-entity read paths. Generated code is never
// hand-edited.
package transport

import (
	"context"
	"errors"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	financeapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/finance"
	"github.com/olegamysk/go-oikumenea/internal/finance/application"
	"github.com/olegamysk/go-oikumenea/internal/finance/domain"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// i18n entity types the translatable catalog names are stored under (localization store).
const (
	entAccountType = "finance_account_type"
	entCardNetwork = "finance_card_network"
)

const (
	readPerm    = string(authzdomain.PermFinanceRead)
	managePerm  = string(authzdomain.PermFinanceManage)
	catalogPerm = string(authzdomain.PermFinanceCatalogManage)
	// The account↔holder ownership link carries its own read code (D-LinkPermissions): finance.read
	// lists the accounts, this discloses WHOSE they are. Same code gates the held_by traversal arm.
	holderReadPerm = string(authzdomain.PermFinanceHolderRead)
)

// FinanceService adapts *application.Service to the generated financeapi.FinanceService interface.
type FinanceService struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the finance application service, the localization service
// (catalog name maps), and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) FinanceService {
	return FinanceService{app: app, loc: loc, pep: enforcer}
}

var _ financeapi.FinanceService = FinanceService{}

// ============================ catalogs ============================

func (s FinanceService) ListAccountTypes(ctx context.Context, token bearertoken.Token) (financeapi.AccountTypeList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return financeapi.AccountTypeList{}, err
	}
	rows, err := s.app.ListAccountTypes(ctx)
	if err != nil {
		return financeapi.AccountTypeList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(rows))
	for _, t := range rows {
		defaults[t.ID] = t.Name
	}
	names, err := s.loc.NamesByID(ctx, entAccountType, defaults)
	if err != nil {
		return financeapi.AccountTypeList{}, s.mapError(ctx, err)
	}
	out := make([]financeapi.AccountType, 0, len(rows))
	for _, t := range rows {
		out = append(out, accountTypeAPI(t, names[t.ID]))
	}
	return financeapi.AccountTypeList{Types: out}, nil
}

func (s FinanceService) UpsertAccountType(ctx context.Context, token bearertoken.Token, req financeapi.UpsertAccountTypeRequest) (financeapi.AccountType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return financeapi.AccountType{}, err
	}
	t, err := s.app.UpsertAccountType(ctx, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return financeapi.AccountType{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entAccountType, t.ID, t.Name)
	if err != nil {
		return financeapi.AccountType{}, s.mapError(ctx, err)
	}
	return accountTypeAPI(t, name), nil
}

func (s FinanceService) ListCardNetworks(ctx context.Context, token bearertoken.Token) (financeapi.CardNetworkList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return financeapi.CardNetworkList{}, err
	}
	rows, err := s.app.ListCardNetworks(ctx)
	if err != nil {
		return financeapi.CardNetworkList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(rows))
	for _, n := range rows {
		defaults[n.ID] = n.Name
	}
	names, err := s.loc.NamesByID(ctx, entCardNetwork, defaults)
	if err != nil {
		return financeapi.CardNetworkList{}, s.mapError(ctx, err)
	}
	out := make([]financeapi.CardNetwork, 0, len(rows))
	for _, n := range rows {
		out = append(out, cardNetworkAPI(n, names[n.ID]))
	}
	return financeapi.CardNetworkList{Networks: out}, nil
}

func (s FinanceService) UpsertCardNetwork(ctx context.Context, token bearertoken.Token, req financeapi.UpsertCardNetworkRequest) (financeapi.CardNetwork, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return financeapi.CardNetwork{}, err
	}
	n, err := s.app.UpsertCardNetwork(ctx, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return financeapi.CardNetwork{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entCardNetwork, n.ID, n.Name)
	if err != nil {
		return financeapi.CardNetwork{}, s.mapError(ctx, err)
	}
	return cardNetworkAPI(n, name), nil
}

// ============================ accounts ============================

func (s FinanceService) CreateAccount(ctx context.Context, token bearertoken.Token, req financeapi.CreateAccountRequest) (financeapi.Account, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return financeapi.Account{}, err
	}
	a, err := s.app.CreateAccount(ctx, req.InstitutionId, req.Iban, strOr(req.Currency), strOr(req.AccountTypeId))
	if err != nil {
		return financeapi.Account{}, s.mapError(ctx, err)
	}
	return s.accountWithLabels(ctx, a)
}

func (s FinanceService) ListAccounts(ctx context.Context, token bearertoken.Token, institutionID *string, pageSize *int, pageToken *string) (financeapi.AccountPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return financeapi.AccountPage{}, err
	}
	limit := pageSizeOr(pageSize)
	rows, err := s.app.ListAccounts(ctx, strOr(institutionID), decodeToken(pageToken), limit)
	if err != nil {
		return financeapi.AccountPage{}, s.mapError(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	out, err := s.accountsWithLabels(ctx, rows)
	if err != nil {
		return financeapi.AccountPage{}, s.mapError(ctx, err)
	}
	page := financeapi.AccountPage{Accounts: out}
	if next != "" {
		page.NextPageToken = &next
	}
	return page, nil
}

func (s FinanceService) GetAccount(ctx context.Context, token bearertoken.Token, accountID string) (financeapi.Account, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return financeapi.Account{}, err
	}
	a, err := s.app.GetAccount(ctx, accountID)
	if err != nil {
		return financeapi.Account{}, s.mapError(ctx, err)
	}
	return s.accountWithLabels(ctx, a)
}

func (s FinanceService) UpdateAccount(ctx context.Context, token bearertoken.Token, accountID string, req financeapi.UpdateAccountRequest) (financeapi.Account, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return financeapi.Account{}, err
	}
	a, err := s.app.UpdateAccount(ctx, accountID, req.Iban, req.Currency, req.AccountTypeId, req.Status)
	if err != nil {
		return financeapi.Account{}, s.mapError(ctx, err)
	}
	return s.accountWithLabels(ctx, a)
}

func (s FinanceService) DeleteAccount(ctx context.Context, token bearertoken.Token, accountID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteAccount(ctx, accountID))
}

// ============================ holders ============================

func (s FinanceService) ListAccountHolders(ctx context.Context, token bearertoken.Token, accountID string) (financeapi.AccountHolderList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, holderReadPerm); err != nil {
		return financeapi.AccountHolderList{}, err
	}
	rows, err := s.app.ListAccountHolders(ctx, accountID)
	if err != nil {
		return financeapi.AccountHolderList{}, s.mapError(ctx, err)
	}
	out, err := s.holdersWithLabels(ctx, rows)
	if err != nil {
		return financeapi.AccountHolderList{}, s.mapError(ctx, err)
	}
	return financeapi.AccountHolderList{Holders: out}, nil
}

func (s FinanceService) AddAccountHolder(ctx context.Context, token bearertoken.Token, accountID string, req financeapi.AddAccountHolderRequest) (financeapi.AccountHolder, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return financeapi.AccountHolder{}, err
	}
	h, err := s.app.AddAccountHolder(ctx, accountID, domain.HolderInput{
		HolderKind: req.HolderKind, HolderID: req.HolderId, Role: strOr(req.Role),
	})
	if err != nil {
		return financeapi.AccountHolder{}, s.mapError(ctx, err)
	}
	out, err := s.holdersWithLabels(ctx, []domain.AccountHolder{h})
	if err != nil {
		return financeapi.AccountHolder{}, s.mapError(ctx, err)
	}
	return out[0], nil
}

func (s FinanceService) EndAccountHolding(ctx context.Context, token bearertoken.Token, holderID string) (financeapi.AccountHolder, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return financeapi.AccountHolder{}, err
	}
	h, err := s.app.EndAccountHolding(ctx, holderID)
	if err != nil {
		return financeapi.AccountHolder{}, s.mapError(ctx, err)
	}
	out, err := s.holdersWithLabels(ctx, []domain.AccountHolder{h})
	if err != nil {
		return financeapi.AccountHolder{}, s.mapError(ctx, err)
	}
	return out[0], nil
}

// ============================ cards ============================

func (s FinanceService) ListCards(ctx context.Context, token bearertoken.Token, accountID string) (financeapi.CardList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return financeapi.CardList{}, err
	}
	rows, err := s.app.ListCards(ctx, accountID)
	if err != nil {
		return financeapi.CardList{}, s.mapError(ctx, err)
	}
	out, err := s.cardsWithLabels(ctx, rows)
	if err != nil {
		return financeapi.CardList{}, s.mapError(ctx, err)
	}
	return financeapi.CardList{Cards: out}, nil
}

func (s FinanceService) AddCard(ctx context.Context, token bearertoken.Token, accountID string, req financeapi.AddCardRequest) (financeapi.Card, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return financeapi.Card{}, err
	}
	c, err := s.app.AddCard(ctx, accountID, req.Pan, strOr(req.NetworkId), req.CardType, req.ExpiryMonth, req.ExpiryYear, strOr(req.CardholderPersonId))
	if err != nil {
		return financeapi.Card{}, s.mapError(ctx, err)
	}
	out, err := s.cardsWithLabels(ctx, []domain.Card{c})
	if err != nil {
		return financeapi.Card{}, s.mapError(ctx, err)
	}
	return out[0], nil
}

func (s FinanceService) GetCard(ctx context.Context, token bearertoken.Token, cardID string) (financeapi.Card, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return financeapi.Card{}, err
	}
	c, err := s.app.GetCard(ctx, cardID)
	if err != nil {
		return financeapi.Card{}, s.mapError(ctx, err)
	}
	out, err := s.cardsWithLabels(ctx, []domain.Card{c})
	if err != nil {
		return financeapi.Card{}, s.mapError(ctx, err)
	}
	return out[0], nil
}

func (s FinanceService) UpdateCard(ctx context.Context, token bearertoken.Token, cardID string, req financeapi.UpdateCardRequest) (financeapi.Card, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return financeapi.Card{}, err
	}
	c, err := s.app.UpdateCard(ctx, cardID, domain.CardUpdate{
		NetworkID: req.NetworkId, CardType: req.CardType, ExpiryMonth: req.ExpiryMonth,
		ExpiryYear: req.ExpiryYear, CardholderPersonID: req.CardholderPersonId, Status: req.Status,
	})
	if err != nil {
		return financeapi.Card{}, s.mapError(ctx, err)
	}
	out, err := s.cardsWithLabels(ctx, []domain.Card{c})
	if err != nil {
		return financeapi.Card{}, s.mapError(ctx, err)
	}
	return out[0], nil
}

func (s FinanceService) DeleteCard(ctx context.Context, token bearertoken.Token, cardID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteCard(ctx, cardID))
}

// ============================ person view ============================

func (s FinanceService) ListPersonAccounts(ctx context.Context, token bearertoken.Token, personID string) (financeapi.PersonAccounts, error) {
	if err := s.pep.RequireAnywhere(ctx, token, holderReadPerm); err != nil {
		return financeapi.PersonAccounts{}, err
	}
	rows, err := s.app.ListPersonAccounts(ctx, personID)
	if err != nil {
		return financeapi.PersonAccounts{}, s.mapError(ctx, err)
	}
	orgIDs, typeIDs := make([]string, 0), make([]string, 0)
	for _, p := range rows {
		orgIDs = append(orgIDs, p.InstitutionID)
		if p.AccountTypeID != "" {
			typeIDs = append(typeIDs, p.AccountTypeID)
		}
	}
	orgs, err := s.app.OrgNamesByIDs(ctx, orgIDs)
	if err != nil {
		return financeapi.PersonAccounts{}, s.mapError(ctx, err)
	}
	types, err := s.app.AccountTypeNamesByIDs(ctx, typeIDs)
	if err != nil {
		return financeapi.PersonAccounts{}, s.mapError(ctx, err)
	}
	out := make([]financeapi.PersonAccount, 0, len(rows))
	for _, p := range rows {
		out = append(out, personAccountAPI(p, orgs[p.InstitutionID], types[p.AccountTypeID]))
	}
	return financeapi.PersonAccounts{Accounts: out}, nil
}

// ============================ label assembly ============================

func (s FinanceService) accountWithLabels(ctx context.Context, a domain.Account) (financeapi.Account, error) {
	out, err := s.accountsWithLabels(ctx, []domain.Account{a})
	if err != nil {
		return financeapi.Account{}, err
	}
	// The single-entity path carries the decrypted IBAN.
	out[0].Iban = emptyToNil(a.IBAN)
	return out[0], nil
}

func (s FinanceService) accountsWithLabels(ctx context.Context, rows []domain.Account) ([]financeapi.Account, error) {
	orgIDs, typeIDs := make([]string, 0), make([]string, 0)
	for _, a := range rows {
		orgIDs = append(orgIDs, a.InstitutionID)
		if a.AccountTypeID != "" {
			typeIDs = append(typeIDs, a.AccountTypeID)
		}
	}
	orgs, err := s.app.OrgNamesByIDs(ctx, orgIDs)
	if err != nil {
		return nil, err
	}
	types, err := s.app.AccountTypeNamesByIDs(ctx, typeIDs)
	if err != nil {
		return nil, err
	}
	out := make([]financeapi.Account, 0, len(rows))
	for _, a := range rows {
		out = append(out, accountAPI(a, orgs[a.InstitutionID], types[a.AccountTypeID]))
	}
	return out, nil
}

func (s FinanceService) holdersWithLabels(ctx context.Context, rows []domain.AccountHolder) ([]financeapi.AccountHolder, error) {
	orgIDs := make([]string, 0)
	for _, h := range rows {
		if h.HolderKind == domain.HolderCompany {
			orgIDs = append(orgIDs, h.HolderID)
		}
	}
	orgs, err := s.app.OrgNamesByIDs(ctx, orgIDs)
	if err != nil {
		return nil, err
	}
	out := make([]financeapi.AccountHolder, 0, len(rows))
	for _, h := range rows {
		out = append(out, holderAPI(h, orgs[h.HolderID]))
	}
	return out, nil
}

func (s FinanceService) cardsWithLabels(ctx context.Context, rows []domain.Card) ([]financeapi.Card, error) {
	networkIDs := make([]string, 0)
	for _, c := range rows {
		if c.NetworkID != "" {
			networkIDs = append(networkIDs, c.NetworkID)
		}
	}
	networks, err := s.app.NetworkNamesByIDs(ctx, networkIDs)
	if err != nil {
		return nil, err
	}
	out := make([]financeapi.Card, 0, len(rows))
	for _, c := range rows {
		out = append(out, cardAPI(c, networks[c.NetworkID]))
	}
	return out, nil
}

// ============================ api mappers ============================

func accountTypeAPI(t domain.AccountType, name map[string]string) financeapi.AccountType {
	return financeapi.AccountType{Id: t.ID, Code: t.Code, Name: name, Status: t.Status, SortOrder: t.SortOrder}
}

func cardNetworkAPI(n domain.CardNetwork, name map[string]string) financeapi.CardNetwork {
	return financeapi.CardNetwork{Id: n.ID, Code: n.Code, Name: name, Status: n.Status, SortOrder: n.SortOrder}
}

func accountAPI(a domain.Account, orgLabel, typeLabel string) financeapi.Account {
	return financeapi.Account{
		Id: a.ID, InstitutionId: a.InstitutionID, InstitutionLabel: emptyToNil(orgLabel),
		Iban: emptyToNil(a.IBAN), Currency: emptyToNil(a.Currency),
		AccountTypeId: emptyToNil(a.AccountTypeID), AccountTypeLabel: emptyToNil(typeLabel),
		Status: a.Status, CreatedAt: datetime.DateTime(a.CreatedAt), UpdatedAt: datetime.DateTime(a.UpdatedAt),
	}
}

func holderAPI(h domain.AccountHolder, orgLabel string) financeapi.AccountHolder {
	out := financeapi.AccountHolder{
		Id: h.ID, AccountId: h.AccountID, HolderKind: h.HolderKind, HolderId: h.HolderID,
		HolderLabel: emptyToNil(orgLabel), Role: h.Role,
		EffectiveFrom: datetime.DateTime(h.EffectiveFrom),
		CreatedAt:     datetime.DateTime(h.CreatedAt), UpdatedAt: datetime.DateTime(h.UpdatedAt),
	}
	if h.EffectiveTo != nil {
		t := datetime.DateTime(*h.EffectiveTo)
		out.EffectiveTo = &t
	}
	return out
}

func cardAPI(c domain.Card, networkLabel string) financeapi.Card {
	return financeapi.Card{
		Id: c.ID, AccountId: c.AccountID, Pan: emptyToNil(c.PAN),
		Bin: emptyToNil(c.BIN), LastFour: emptyToNil(c.LastFour),
		NetworkId: emptyToNil(c.NetworkID), NetworkLabel: emptyToNil(networkLabel),
		CardType: c.CardType, ExpiryMonth: c.ExpiryMonth, ExpiryYear: c.ExpiryYear,
		CardholderPersonId: emptyToNil(c.CardholderPersonID),
		Status:             c.Status, CreatedAt: datetime.DateTime(c.CreatedAt), UpdatedAt: datetime.DateTime(c.UpdatedAt),
	}
}

func personAccountAPI(p domain.PersonAccount, orgLabel, typeLabel string) financeapi.PersonAccount {
	return financeapi.PersonAccount{
		Id: p.ID, InstitutionId: p.InstitutionID, InstitutionLabel: emptyToNil(orgLabel),
		Currency: emptyToNil(p.Currency), AccountTypeLabel: emptyToNil(typeLabel),
		Role: p.Role, Status: p.Status, CreatedAt: datetime.DateTime(p.CreatedAt),
	}
}

// ============================ helpers ============================

func (s FinanceService) nameMap(ctx context.Context, entityType, id, def string) (map[string]string, error) {
	m, err := s.loc.NamesByID(ctx, entityType, map[string]string{id: def})
	if err != nil {
		return nil, err
	}
	return m[id], nil
}

func (s FinanceService) mapError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrAccountNotFound):
		return financeapi.NewAccountNotFound("")
	case errors.Is(err, domain.ErrCardNotFound):
		return financeapi.NewCardNotFound("")
	case errors.Is(err, domain.ErrHolderNotFound):
		return financeapi.NewHolderNotFound("")
	case errors.Is(err, domain.ErrConflict):
		return financeapi.NewConflict("identifier already exists in scope")
	case errors.Is(err, domain.ErrInvalid):
		return financeapi.NewInvalid("invalid request or unknown reference")
	}
	return werror.WrapWithContextParams(ctx, err, "finance operation failed")
}

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---- pagination tokens (opaque base64 of the last id) ----

// pageSizePolicy mirrors the owning application service's clamp, applied at the wire edge over the
// optional Conjure arg (M56 / pkg/listing).
var pageSizePolicy = listing.PageSize{Default: 50, Max: 200}

func pageSizeOr(p *int) int { return pageSizePolicy.ResolvePtr(p) }

// decodeToken/encodeToken are the opaque keyset cursor over the last row's RID, delegated to the
// shared pkg/listing codec (M56). These endpoints previously emitted base64 StdEncoding, whose
// `+`, `/` and `=` are NOT URL-safe in a query parameter (a `+` decodes to a space, corrupting the
// cursor); listing.EncodeCursor emits RawURL, and its decode stays tolerant of the old alphabet so
// tokens issued before the upgrade keep working. An undecodable token still yields "" — restarting
// at the first page — preserving this transport's existing behaviour.
func decodeToken(p *string) string {
	id, err := listing.DecodeCursorPtr(p)
	if err != nil {
		return ""
	}
	return id
}

func encodeToken(id string) string { return listing.EncodeCursor(id) }
