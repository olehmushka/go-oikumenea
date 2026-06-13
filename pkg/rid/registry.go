package rid

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// The RID registries — the Go mirror of oikumenea.platform_rid_services / platform_rid_types
// (migration 0000). AssertMatches verifies the two agree at boot so they cannot drift. The
// authoritative *list* of types is docs/ontology-mapping.md; the numeric codes are assigned here and
// in the migration together.

var serviceNames = map[int]string{
	SvcPlatform:   "platform",
	SvcI18n:       "i18n",
	SvcAudit:      "audit",
	SvcTenant:     "tenant",
	SvcRank:       "rank",
	SvcPerson:     "person",
	SvcMembership: "membership",
	SvcAuthz:      "authz",
	SvcAccount:    "account",
	SvcDocument:   "document",
	SvcOrder:      "order",
}

type typeKey struct {
	service int
	kind    int
	code    int
}

// typeNames maps (service, kind, type_code) -> type name. Mirrors the platform_rid_types seed.
var typeNames = map[typeKey]string{
	// i18n
	{SvcI18n, int(KindObject), 1}: "translation",
	// tenant
	{SvcTenant, int(KindObject), 1}: "unit",
	{SvcTenant, int(KindObject), 2}: "graph",
	{SvcTenant, int(KindObject), 3}: "unit_lifecycle_event",
	{SvcTenant, int(KindLink), 1}:   "parent_of",
	// rank
	{SvcRank, int(KindObject), 1}: "system",
	{SvcRank, int(KindObject), 2}: "category",
	{SvcRank, int(KindObject), 3}: "type",
	{SvcRank, int(KindObject), 4}: "rank",
	// person objects
	{SvcPerson, int(KindObject), 1}:  "person",
	{SvcPerson, int(KindObject), 2}:  "name_variant",
	{SvcPerson, int(KindObject), 3}:  "citizenship",
	{SvcPerson, int(KindObject), 4}:  "residence",
	{SvcPerson, int(KindObject), 5}:  "email",
	{SvcPerson, int(KindObject), 6}:  "phone",
	{SvcPerson, int(KindObject), 7}:  "call_sign",
	{SvcPerson, int(KindObject), 8}:  "messenger_link",
	{SvcPerson, int(KindObject), 9}:  "social_account",
	{SvcPerson, int(KindObject), 10}: "social_handle",
	// person links
	{SvcPerson, int(KindLink), 1}: "holds_rank",
	{SvcPerson, int(KindLink), 2}: "partnered_with",
	{SvcPerson, int(KindLink), 3}: "kin_parent_of",
	{SvcPerson, int(KindLink), 4}: "guardian_of",
	{SvcPerson, int(KindLink), 5}: "sponsor_of",
	{SvcPerson, int(KindLink), 6}: "next_of_kin",
	{SvcPerson, int(KindLink), 7}: "associated_with",
	// membership
	{SvcMembership, int(KindObject), 1}: "position",
	{SvcMembership, int(KindLink), 1}:   "member_of",
	// authz
	{SvcAuthz, int(KindObject), 1}: "role",
	{SvcAuthz, int(KindLink), 1}:   "has_role",
	{SvcAuthz, int(KindLink), 2}:   "instance_admin",
	// account
	{SvcAccount, int(KindObject), 1}: "account",
	{SvcAccount, int(KindObject), 2}: "external_identity",
	// document
	{SvcDocument, int(KindObject), 1}: "document_type",
	{SvcDocument, int(KindObject), 2}: "document",
	{SvcDocument, int(KindObject), 3}: "personal_code",
	// order
	{SvcOrder, int(KindObject), 1}: "order_type",
	{SvcOrder, int(KindObject), 2}: "order",
	{SvcOrder, int(KindObject), 3}: "order_item",
}

// Bare person link-type names (the dispatch tokens), derived from the registry above.
const (
	LinkHoldsRank    = "holds_rank"
	LinkPartnership  = "partnered_with"
	LinkKinship      = "kin_parent_of"
	LinkGuardianship = "guardian_of"
	LinkSponsorship  = "sponsor_of"
	LinkNextOfKin    = "next_of_kin"
	LinkAssociation  = "associated_with"
)

// Querier is the minimal pgx surface AssertMatches needs (satisfied by *pgxpool.Pool / pgx.Conn / tx).
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// AssertMatches verifies the Go registries equal the seeded SQL registries (services + types), so a
// drift between migration 0000 and this package fails fast at boot rather than minting wrong RIDs.
func AssertMatches(ctx context.Context, q Querier) error {
	// services
	srvRows, err := q.Query(ctx, "SELECT code, module FROM oikumenea.platform_rid_services")
	if err != nil {
		return fmt.Errorf("rid: load services: %w", err)
	}
	dbSvc := map[int]string{}
	for srvRows.Next() {
		var code int
		var name string
		if err := srvRows.Scan(&code, &name); err != nil {
			srvRows.Close()
			return fmt.Errorf("rid: scan service: %w", err)
		}
		dbSvc[code] = name
	}
	srvRows.Close()
	if err := srvRows.Err(); err != nil {
		return fmt.Errorf("rid: services rows: %w", err)
	}
	if len(dbSvc) != len(serviceNames) {
		return fmt.Errorf("rid: service registry size mismatch: db=%d go=%d", len(dbSvc), len(serviceNames))
	}
	for code, name := range dbSvc {
		if serviceNames[code] != name {
			return fmt.Errorf("rid: service %d = %q in db but %q in go", code, name, serviceNames[code])
		}
	}
	// types
	typRows, err := q.Query(ctx, "SELECT service_code, kind, type_code, type_name FROM oikumenea.platform_rid_types WHERE kind <> 3")
	if err != nil {
		return fmt.Errorf("rid: load types: %w", err)
	}
	count := 0
	for typRows.Next() {
		var svc, kind, code int
		var name string
		if err := typRows.Scan(&svc, &kind, &code, &name); err != nil {
			typRows.Close()
			return fmt.Errorf("rid: scan type: %w", err)
		}
		if got := typeNames[typeKey{service: svc, kind: kind, code: code}]; got != name {
			typRows.Close()
			return fmt.Errorf("rid: type (%d,%d,%d) = %q in db but %q in go", svc, kind, code, name, got)
		}
		count++
	}
	typRows.Close()
	if err := typRows.Err(); err != nil {
		return fmt.Errorf("rid: types rows: %w", err)
	}
	if count != len(typeNames) {
		return fmt.Errorf("rid: type registry size mismatch: db=%d go=%d", count, len(typeNames))
	}
	return nil
}
