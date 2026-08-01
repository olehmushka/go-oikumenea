// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package application is the finance module's orchestrator (D-Finance, M44): audited writes over the
// account-type / card-network catalogs, bank accounts (envelope-encrypted IBAN), the polymorphic holder
// link, and payment cards (envelope-encrypted PAN + clear BIN/last-4). The sensitive IBAN/PAN are
// validated on the plaintext (D-PersonalCodes: IBAN ISO-13616, PAN Luhn), envelope-encrypted before
// persistence and decrypted on read (D-CryptoProvider) — the repository sees only ciphertext. Every
// write runs in a transaction that also records the audit Action (D-Audit); reads run on the pool.
// Finance is authoritative first-party directory data, so writes are recorded under a `system` actor
// (mirroring vehicle/company).
package application

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/finance/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/olegamysk/go-oikumenea/pkg/personalcode"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

const (
	auditSubsystem  = "finance-admin"
	defaultPageSize = 50
	maxPageSize     = 200

	schemeIBAN = "iban"
	schemePAN  = "pan"
)

// RepositoryFactory binds a domain.Repository to a command surface (pool for reads, tx for writes).
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the finance application service.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
	cipher  *crypto.Cipher
	codes   *personalcode.Registry
	// labeler resolves ref-bucket RIDs to locale->text names; injected at the composition root. ONE
	// labeler serves both object types this module owns — it dispatches on the ref TYPE token, and
	// account's `organization` buckets and card's `card_network` buckets are different tokens.
	labeler stats.Labeler
}

// SetBucketLabeler injects the composition root's ref-bucket resolver (bank organization, account-type
// and card-network RIDs to locale->text names). Set once at boot; a nil labeler simply leaves buckets
// unlabelled, exactly as an unresolvable id does.
func (s *Service) SetBucketLabeler(l stats.Labeler) { s.labeler = l }

// NewService wires the service with the pool, repository factory, audit service, the envelope cipher
// (D-CryptoProvider), and the personal-code validator registry (D-PersonalCodes, for IBAN/PAN).
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, cipher *crypto.Cipher, codes *personalcode.Registry) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit, cipher: cipher, codes: codes}
}

// ============================ catalogs ============================

func (s *Service) ListAccountTypes(ctx context.Context) ([]domain.AccountType, error) {
	return s.newRepo(s.querier(ctx)).ListAccountTypes(ctx)
}

func (s *Service) UpsertAccountType(ctx context.Context, code, name string, sortOrder *int) (domain.AccountType, error) {
	var out domain.AccountType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertAccountType(ctx, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "finance.account-type.upsert", v.ID, v)
	})
	return out, err
}

func (s *Service) ListCardNetworks(ctx context.Context) ([]domain.CardNetwork, error) {
	return s.newRepo(s.querier(ctx)).ListCardNetworks(ctx)
}

func (s *Service) UpsertCardNetwork(ctx context.Context, code, name string, sortOrder *int) (domain.CardNetwork, error) {
	var out domain.CardNetwork
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertCardNetwork(ctx, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "finance.card-network.upsert", v.ID, v)
	})
	return out, err
}

// ============================ accounts ============================

// CreateAccount validates the institution, validates + encrypts the IBAN, and stores ciphertext + blind
// index. The returned Account carries the just-validated plaintext IBAN.
func (s *Service) CreateAccount(ctx context.Context, institutionID, iban, currency, accountTypeID string) (domain.Account, error) {
	normIBAN, err := s.validateCode(schemeIBAN, iban)
	if err != nil {
		return domain.Account{}, err
	}
	sealed, blind, err := s.encrypt(ctx, normIBAN)
	if err != nil {
		return domain.Account{}, err
	}
	var out domain.StoredAccount
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		ok, err := repo.OrgExists(ctx, institutionID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrInvalid
		}
		created, err := repo.InsertAccount(ctx, domain.AccountInput{
			InstitutionID:  institutionID,
			IBANCiphertext: sealed.Ciphertext,
			IBANWrappedDEK: sealed.WrappedDEK,
			KeyRef:         sealed.KeyRef,
			IBANBlindIndex: blind,
			Currency:       currency,
			AccountTypeID:  accountTypeID,
		})
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "finance.account.create", created.ID,
			map[string]any{"id": created.ID, "institutionId": created.InstitutionID})
	})
	if err != nil {
		return domain.Account{}, err
	}
	return toAccount(out, normIBAN), nil
}

// GetAccount returns the account with the decrypted IBAN (for authorized callers; transport gates).
func (s *Service) GetAccount(ctx context.Context, id string) (domain.Account, error) {
	stored, err := s.newRepo(s.querier(ctx)).GetAccount(ctx, id)
	if err != nil {
		return domain.Account{}, mapNotFound(err, domain.ErrAccountNotFound)
	}
	iban, err := s.decrypt(ctx, stored.IBANCiphertext, stored.IBANWrappedDEK, stored.KeyRef)
	if err != nil {
		return domain.Account{}, err
	}
	return toAccount(stored, iban), nil
}

// ListAccounts returns accounts WITHOUT the decrypted IBAN (never listed).
func (s *Service) ListAccounts(ctx context.Context, after string, f domain.AccountFilter, pageSize int) ([]domain.Account, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	stored, err := s.newRepo(s.querier(ctx)).ListAccounts(ctx, after, f, clampPageSize(pageSize)+1)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Account, 0, len(stored))
	for _, a := range stored {
		out = append(out, toAccount(a, ""))
	}
	return out, nil
}

// UpdateAccount changes currency/accountTypeId/status and optionally re-keys the IBAN.
func (s *Service) UpdateAccount(ctx context.Context, id string, iban, currency, accountTypeID, status *string) (domain.Account, error) {
	up := domain.AccountUpdate{Currency: currency, AccountTypeID: accountTypeID, Status: status}
	plain := ""
	if iban != nil {
		normIBAN, err := s.validateCode(schemeIBAN, *iban)
		if err != nil {
			return domain.Account{}, err
		}
		sealed, blind, err := s.encrypt(ctx, normIBAN)
		if err != nil {
			return domain.Account{}, err
		}
		up.RekeyIBAN = true
		up.IBANCiphertext, up.IBANWrappedDEK, up.KeyRef, up.IBANBlindIndex = sealed.Ciphertext, sealed.WrappedDEK, sealed.KeyRef, blind
		plain = normIBAN
	}
	var out domain.StoredAccount
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateAccount(ctx, id, up)
		if err != nil {
			return mapNotFound(err, domain.ErrAccountNotFound)
		}
		out = v
		return s.record(ctx, tx, "finance.account.update", id,
			map[string]any{"id": id, "ibanRekeyed": iban != nil})
	})
	if err != nil {
		return domain.Account{}, err
	}
	if plain == "" {
		plain, err = s.decrypt(ctx, out.IBANCiphertext, out.IBANWrappedDEK, out.KeyRef)
		if err != nil {
			return domain.Account{}, err
		}
	}
	return toAccount(out, plain), nil
}

func (s *Service) DeleteAccount(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteAccount(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrAccountNotFound
		}
		return s.record(ctx, tx, "finance.account.delete", id, nil)
	})
}

// ============================ holders ============================

func (s *Service) ListAccountHolders(ctx context.Context, accountID string) ([]domain.AccountHolder, error) {
	if _, err := s.newRepo(s.querier(ctx)).GetAccount(ctx, accountID); err != nil {
		return nil, mapNotFound(err, domain.ErrAccountNotFound)
	}
	return s.newRepo(s.querier(ctx)).ListHoldersByAccount(ctx, accountID)
}

// AddAccountHolder adds a person|company holder to the account, validating the account and the holder.
func (s *Service) AddAccountHolder(ctx context.Context, accountID string, in domain.HolderInput) (domain.AccountHolder, error) {
	if err := in.Validate(); err != nil {
		return domain.AccountHolder{}, err
	}
	var out domain.AccountHolder
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetAccount(ctx, accountID); err != nil {
			return mapNotFound(err, domain.ErrAccountNotFound)
		}
		ok, err := s.holderExists(ctx, repo, in.HolderKind, in.HolderID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrInvalid
		}
		h, err := repo.InsertHolder(ctx, accountID, in)
		if err != nil {
			return err
		}
		out = h
		return s.record(ctx, tx, "finance.holder.add", h.ID, h)
	})
	return out, err
}

func (s *Service) EndAccountHolding(ctx context.Context, holderID string) (domain.AccountHolder, error) {
	var out domain.AccountHolder
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		h, err := s.newRepo(tx).EndHolder(ctx, holderID)
		if err != nil {
			return mapNotFound(err, domain.ErrHolderNotFound)
		}
		out = h
		return s.record(ctx, tx, "finance.holder.end", holderID, h)
	})
	return out, err
}

// ============================ cards ============================

// ListAccountCards is the per-account view, renamed from ListCards in M58 ticket 3 when the plain
// name went to the instance-wide registry below (the contract made the same move: listAccountCards
// sits beside listAccountHolders, and the HTTP path is unchanged).
func (s *Service) ListAccountCards(ctx context.Context, accountID string) ([]domain.Card, error) {
	if _, err := s.newRepo(s.querier(ctx)).GetAccount(ctx, accountID); err != nil {
		return nil, mapNotFound(err, domain.ErrAccountNotFound)
	}
	stored, err := s.newRepo(s.querier(ctx)).ListCardsByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Card, 0, len(stored))
	for _, c := range stored {
		out = append(out, toCard(c, "")) // PAN not decrypted in a list
	}
	return out, nil
}

// ListCards pages the INSTANCE-WIDE card registry (M58 ticket 3) — the collection-level list the card
// dashboard describes. Cards were previously reachable only through their account, so there was no
// collection for a facet vocabulary to page or count.
//
// toCard is called with an empty PAN, exactly as the per-account list does: this endpoint widens the
// SCOPE of a read `finance.read` already permits and discloses no field the per-account list did not
// already return. The PAN is decrypted by GetCard alone (PCI-DSS Req 3; D-DataScope CDE scope).
func (s *Service) ListCards(ctx context.Context, after string, f domain.CardFilter, pageSize int) ([]domain.Card, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	stored, err := s.newRepo(s.querier(ctx)).ListCards(ctx, after, f, clampPageSize(pageSize)+1)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Card, 0, len(stored))
	for _, c := range stored {
		out = append(out, toCard(c, ""))
	}
	return out, nil
}

// AccountStats and CardStats are the dashboard halves of the facet vocabulary (M58 ticket 3 /
// D-ObjectFacets): the same filters their lists take, aggregated instead of paged.
//
// Both call stats.Compute with isAdmin=true, which is the arm convention's way of saying "no
// visibility predicate" — and here that is a statement of fact rather than a privilege escalation.
// Neither finance_accounts nor finance_cards carries row-level security or a unit reach; the
// transport has already required `finance.read`, which is the whole gate on the list endpoints too,
// so any caller who reaches these lines may read every row they count. A scoped arm would have
// nothing to narrow.
func (s *Service) AccountStats(ctx context.Context, f domain.AccountFilter, sel stats.Selection) (stats.Result, error) {
	if err := f.Validate(); err != nil {
		return stats.Result{}, err
	}
	return stats.Compute(ctx, s.labeler, sel, true, "", func(string) ([]stats.Group, error) {
		return s.newRepo(s.querier(ctx)).AccountStats(ctx, f, sel)
	})
}

func (s *Service) CardStats(ctx context.Context, f domain.CardFilter, sel stats.Selection) (stats.Result, error) {
	if err := f.Validate(); err != nil {
		return stats.Result{}, err
	}
	return stats.Compute(ctx, s.labeler, sel, true, "", func(string) ([]stats.Group, error) {
		return s.newRepo(s.querier(ctx)).CardStats(ctx, f, sel)
	})
}

// AddCard validates + encrypts the PAN (deriving BIN + last-4 from the normalized digits), guards the
// card type + optional cardholder, and stores the card under the account.
func (s *Service) AddCard(ctx context.Context, accountID, pan, networkID, cardType string, expiryMonth, expiryYear *int, cardholderPersonID string) (domain.Card, error) {
	if !domain.ValidCardType(cardType) {
		return domain.Card{}, domain.ErrInvalid
	}
	normPAN, err := s.validateCode(schemePAN, pan)
	if err != nil {
		return domain.Card{}, err
	}
	bin, last4 := personalcode.SplitPAN(normPAN)
	sealed, blind, err := s.encrypt(ctx, normPAN)
	if err != nil {
		return domain.Card{}, err
	}
	var out domain.StoredCard
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetAccount(ctx, accountID); err != nil {
			return mapNotFound(err, domain.ErrAccountNotFound)
		}
		if cardholderPersonID != "" {
			ok, err := repo.PersonExists(ctx, cardholderPersonID)
			if err != nil {
				return err
			}
			if !ok {
				return domain.ErrInvalid
			}
		}
		created, err := repo.InsertCard(ctx, accountID, domain.CardInput{
			PANCiphertext:      sealed.Ciphertext,
			PANWrappedDEK:      sealed.WrappedDEK,
			KeyRef:             sealed.KeyRef,
			PANBlindIndex:      blind,
			BIN:                bin,
			LastFour:           last4,
			NetworkID:          networkID,
			CardType:           cardType,
			ExpiryMonth:        expiryMonth,
			ExpiryYear:         expiryYear,
			CardholderPersonID: cardholderPersonID,
		})
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "finance.card.add", created.ID,
			map[string]any{"id": created.ID, "accountId": accountID, "bin": bin, "lastFour": last4})
	})
	if err != nil {
		return domain.Card{}, err
	}
	return toCard(out, normPAN), nil
}

// GetCard returns the card with the decrypted PAN (for authorized callers; transport gates).
func (s *Service) GetCard(ctx context.Context, id string) (domain.Card, error) {
	stored, err := s.newRepo(s.querier(ctx)).GetCard(ctx, id)
	if err != nil {
		return domain.Card{}, mapNotFound(err, domain.ErrCardNotFound)
	}
	pan, err := s.decrypt(ctx, stored.PANCiphertext, stored.PANWrappedDEK, stored.KeyRef)
	if err != nil {
		return domain.Card{}, err
	}
	return toCard(stored, pan), nil
}

func (s *Service) UpdateCard(ctx context.Context, id string, up domain.CardUpdate) (domain.Card, error) {
	if up.CardType != nil && !domain.ValidCardType(*up.CardType) {
		return domain.Card{}, domain.ErrInvalid
	}
	var out domain.StoredCard
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateCard(ctx, id, up)
		if err != nil {
			return mapNotFound(err, domain.ErrCardNotFound)
		}
		out = v
		return s.record(ctx, tx, "finance.card.update", id, map[string]any{"id": id})
	})
	if err != nil {
		return domain.Card{}, err
	}
	return toCard(out, ""), nil
}

func (s *Service) DeleteCard(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteCard(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrCardNotFound
		}
		return s.record(ctx, tx, "finance.card.delete", id, nil)
	})
}

// ============================ person view / purge ============================

func (s *Service) ListPersonAccounts(ctx context.Context, personID string) ([]domain.PersonAccount, error) {
	return s.newRepo(s.querier(ctx)).ListAccountsByPersonHolder(ctx, personID)
}

// ErasePersonAccounts is the person-purge erasure path (D-Finance): it crypto-erases the accounts (+
// cards) the person SOLELY holds and soft-deletes their holder edges. Company-held (and joint) accounts
// survive. Triggered by the PersonPurged event (SubscribePersonPurge); also exercised directly.
func (s *Service) ErasePersonAccounts(ctx context.Context, personID string) (int64, error) {
	var n int64
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.erasePersonAccountsTx(ctx, tx, personID)
		n = v
		return err
	})
	return n, err
}

// erasePersonAccountsTx is the body of the person-purge erasure, run in a caller-supplied transaction so
// it executes either standalone (ErasePersonAccounts) or inside the person-purge tx as the PersonPurged
// subscriber (SubscribePersonPurge). The audit row is written only when something was erased.
func (s *Service) erasePersonAccountsTx(ctx context.Context, tx pgx.Tx, personID string) (int64, error) {
	n, err := s.newRepo(tx).ErasePersonHoldings(ctx, personID)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		if err := s.record(ctx, tx, "finance.holdings.erase", personID, map[string]int64{"accountsErased": n}); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// ============================ label helpers (transport) ============================

func (s *Service) OrgNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return s.newRepo(s.querier(ctx)).OrgNamesByIDs(ctx, ids)
}

func (s *Service) AccountTypeNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return s.newRepo(s.querier(ctx)).AccountTypeNamesByIDs(ctx, ids)
}

func (s *Service) NetworkNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return s.newRepo(s.querier(ctx)).NetworkNamesByIDs(ctx, ids)
}

// ============================ crypto + validation ============================

// validateCode applies the IBAN/PAN scheme validator (D-PersonalCodes); an invalid value → ErrInvalid.
func (s *Service) validateCode(scheme, value string) (string, error) {
	res, err := s.codes.Validate(scheme, value, "")
	if err != nil {
		if errors.Is(err, personalcode.ErrInvalid) {
			return "", domain.ErrInvalid
		}
		return "", err
	}
	return res.Normalized, nil
}

func (s *Service) encrypt(ctx context.Context, normalized string) (crypto.Sealed, []byte, error) {
	sealed, err := s.cipher.Seal(ctx, []byte(normalized))
	if err != nil {
		return crypto.Sealed{}, nil, err
	}
	return sealed, s.cipher.BlindIndex([]byte(normalized)), nil
}

// decrypt recovers a plaintext value; a crypto-erased row (no ciphertext) yields "" (the tombstone).
func (s *Service) decrypt(ctx context.Context, ciphertext, wrappedDEK []byte, keyRef string) (string, error) {
	if len(ciphertext) == 0 || len(wrappedDEK) == 0 {
		return "", nil
	}
	plain, err := s.cipher.Open(ctx, crypto.Sealed{Ciphertext: ciphertext, WrappedDEK: wrappedDEK, KeyRef: keyRef})
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// holderExists validates a polymorphic holder id against its kind.
func (s *Service) holderExists(ctx context.Context, repo domain.Repository, kind, id string) (bool, error) {
	if kind == domain.HolderCompany {
		return repo.OrgExists(ctx, id)
	}
	return repo.PersonExists(ctx, id)
}

// ============================ mappers ============================

func toAccount(a domain.StoredAccount, iban string) domain.Account {
	return domain.Account{
		ID: a.ID, InstitutionID: a.InstitutionID, IBAN: iban,
		Currency: a.Currency, AccountTypeID: a.AccountTypeID, Status: a.Status,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func toCard(c domain.StoredCard, pan string) domain.Card {
	return domain.Card{
		ID: c.ID, AccountID: c.AccountID, PAN: pan, BIN: c.BIN, LastFour: c.LastFour,
		NetworkID: c.NetworkID, CardType: c.CardType, ExpiryMonth: c.ExpiryMonth, ExpiryYear: c.ExpiryYear,
		CardholderPersonID: c.CardholderPersonID, Status: c.Status, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

// ============================ infra ============================

// pageSizePolicy is this module's page-size policy, clamped through the shared kernel (M56 /
// pkg/listing) instead of a local copy.
var pageSizePolicy = listing.PageSize{Default: defaultPageSize, Max: maxPageSize}

func clampPageSize(n int) int { return pageSizePolicy.Resolve(n) }

func (s *Service) querier(ctx context.Context) db.Querier {
	return db.RequestQuerier(ctx, s.pool)
}

func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.querier(ctx).Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// record mints an Action RID (finance service=19, kind=action=3, type=0) in the caller's transaction and
// writes the audit row on it (D-Audit). Finance data is authoritative first-party directory data → system actor.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetID string, after any) error {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(19, 3, 0)").Scan(&rid); err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: "finance",
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

func mapNotFound(err error, sentinel error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sentinel
	}
	return err
}

func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

func toJSON(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
