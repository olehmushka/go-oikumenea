//go:build integration

// Integration tests for the person module against a real Postgres (M5 exit criteria, D-PersonGlobal /
// D-PersonNamesCLDR / D-Geo / D-PersonReadScope / D-Audit):
//   - a person is created with no account and no unit, and reads back with its children;
//   - the optional external code is unique among active persons;
//   - a person may hold several citizenships (one active per country) with a single primary;
//   - rank is set/cleared; an unknown rank/country/locale is rejected via the DB FKs;
//   - name variants are unique per (person, locale);
//   - deactivate -> reactivate is reversible; purge before the grace window is refused, and after it
//     NULLs the PII while keeping the id tombstone;
//   - a create write + its audit row share one transaction.
//
// Run against a throwaway DB that has the migrations applied:
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/postgres?sslmode=disable" \
//	  go test -tags integration ./internal/person/...
package person_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/person/adapters"
	"github.com/olegamysk/go-oikumenea/internal/person/application"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/postgres?sslmode=disable"

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// newService builds the person application service directly (bypassing Register) with a fixed purge
// grace window in hours.
func newService(t *testing.T, graceHours int) (*application.Service, *pgxpool.Pool) {
	t.Helper()
	pool := newPool(t)
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	repoFor := func(conn pdb.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	return application.NewService(pool, repoFor, audit, func() int { return graceHours }), pool
}

func code(t *testing.T, prefix string) string {
	t.Helper()
	return prefix + "-" + uuid.NewString()[:8]
}

// seedRank inserts a category -> type -> rank chain directly and returns the rank RID.
func seedRank(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var catID, typeID, rankID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_categories (code, name, sort_order) VALUES ($1, 'Cat', 0) RETURNING id`,
		code(t, "cat")).Scan(&catID); err != nil {
		t.Fatalf("seed category: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_types (category_id, code, name, sort_order) VALUES ($1, $2, 'Typ', 0) RETURNING id`,
		catID, code(t, "typ")).Scan(&typeID); err != nil {
		t.Fatalf("seed type: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_ranks (type_id, code, name, sort_order) VALUES ($1, $2, 'Rnk', 0) RETURNING id`,
		typeID, code(t, "rnk")).Scan(&rankID); err != nil {
		t.Fatalf("seed rank: %v", err)
	}
	return rankID
}

func newPerson(t *testing.T, svc *application.Service, display string) domain.Person {
	t.Helper()
	p, err := svc.CreatePerson(context.Background(), domain.Person{Name: domain.Name{DisplayName: display}})
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	return p
}

// TestCreateAndReadAccountless creates a person with no account/unit and reads it back.
func TestCreateAndReadAccountless(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)

	created, err := svc.CreatePerson(ctx, domain.Person{
		Name:      domain.Name{DisplayName: "Тарас Григорович Шевченко", Given: "Тарас", Given2: "Григорович", Surname: "Шевченко"},
		Birthdate: "1990-05-02",
		Sex:       "male",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Status != domain.StatusActive {
		t.Fatalf("status = %q, want active", created.Status)
	}
	got, err := svc.GetPerson(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Given2 != "Григорович" {
		t.Fatalf("given2 = %q, want the по-батькові", got.Given2)
	}
	if got.Sex != "male" || got.Birthdate != "1990-05-02" {
		t.Fatalf("bio not round-tripped: sex=%q birthdate=%q", got.Sex, got.Birthdate)
	}
}

// TestCodeUniqueAmongActive rejects a duplicate external code.
func TestCodeUniqueAmongActive(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)

	c := code(t, "svc")
	if _, err := svc.CreatePerson(ctx, domain.Person{Code: c, Name: domain.Name{DisplayName: "First"}}); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := svc.CreatePerson(ctx, domain.Person{Code: c, Name: domain.Name{DisplayName: "Second"}})
	if !errors.Is(err, domain.ErrCodeConflict) {
		t.Fatalf("want ErrCodeConflict, got %v", err)
	}
}

// TestRankAssignment sets and clears a rank, and rejects an unknown rank.
func TestRankAssignment(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t, 720)
	p := newPerson(t, svc, "Ranked Person")
	rankID := seedRank(t, pool)

	if _, err := svc.SetRank(ctx, p.ID, &rankID); err != nil {
		t.Fatalf("set rank: %v", err)
	}
	got, _ := svc.GetPerson(ctx, p.ID)
	if got.RankID != rankID {
		t.Fatalf("rankID = %q, want %q", got.RankID, rankID)
	}
	// clear
	if _, err := svc.SetRank(ctx, p.ID, nil); err != nil {
		t.Fatalf("clear rank: %v", err)
	}
	if got, _ := svc.GetPerson(ctx, p.ID); got.RankID != "" {
		t.Fatalf("rankID = %q, want empty after clear", got.RankID)
	}
	// unknown rank
	bogus := "urn:oikumenea:rank:local:rank:" + uuid.NewString()
	if _, err := svc.SetRank(ctx, p.ID, &bogus); !errors.Is(err, domain.ErrUnknownRank) {
		t.Fatalf("unknown rank: want ErrUnknownRank, got %v", err)
	}
}

// TestCitizenships holds several citizenships with one active per country and a single primary.
func TestCitizenships(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)
	p := newPerson(t, svc, "Multi National")

	if _, err := svc.UpsertCitizenship(ctx, domain.Citizenship{PersonID: p.ID, Country: "UA", Basis: "birth", IsPrimary: true}); err != nil {
		t.Fatalf("add UA: %v", err)
	}
	if _, err := svc.UpsertCitizenship(ctx, domain.Citizenship{PersonID: p.ID, Country: "PL", Basis: "naturalization", IsPrimary: true}); err != nil {
		t.Fatalf("add PL: %v", err)
	}
	cs, err := svc.ListCitizenships(ctx, p.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("citizenships = %d, want 2", len(cs))
	}
	primaries := 0
	for _, c := range cs {
		if c.IsPrimary {
			primaries++
		}
	}
	if primaries != 1 {
		t.Fatalf("primary citizenships = %d, want exactly 1", primaries)
	}
	// re-upsert UA: still one active UA row (no duplicate).
	if _, err := svc.UpsertCitizenship(ctx, domain.Citizenship{PersonID: p.ID, Country: "UA", Basis: "birth"}); err != nil {
		t.Fatalf("re-upsert UA: %v", err)
	}
	if cs, _ := svc.ListCitizenships(ctx, p.ID); len(cs) != 2 {
		t.Fatalf("after re-upsert, citizenships = %d, want 2", len(cs))
	}
	// unknown country.
	if _, err := svc.UpsertCitizenship(ctx, domain.Citizenship{PersonID: p.ID, Country: "ZZ", Basis: "other"}); !errors.Is(err, domain.ErrUnknownCountry) {
		t.Fatalf("unknown country: want ErrUnknownCountry, got %v", err)
	}
	// remove PL.
	if err := svc.DeleteCitizenship(ctx, p.ID, "PL"); err != nil {
		t.Fatalf("delete PL: %v", err)
	}
	if cs, _ := svc.ListCitizenships(ctx, p.ID); len(cs) != 1 {
		t.Fatalf("after delete, citizenships = %d, want 1", len(cs))
	}
}

// TestNameVariants are unique per (person, locale); an unknown locale is rejected.
func TestNameVariants(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)
	p := newPerson(t, svc, "Тарас Шевченко")

	if _, err := svc.UpsertNameVariant(ctx, domain.NameVariant{PersonID: p.ID, Locale: "eng", Name: domain.Name{DisplayName: "John Doe"}, IsPrimary: true}); err != nil {
		t.Fatalf("add eng: %v", err)
	}
	// re-upsert eng updates in place (no duplicate, no conflict).
	if _, err := svc.UpsertNameVariant(ctx, domain.NameVariant{PersonID: p.ID, Locale: "eng", Name: domain.Name{DisplayName: "John V. Doe"}}); err != nil {
		t.Fatalf("re-upsert eng: %v", err)
	}
	vs, err := svc.ListNameVariants(ctx, p.ID)
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if len(vs) != 1 || vs[0].DisplayName != "John V. Doe" {
		t.Fatalf("variants = %+v, want one updated eng variant", vs)
	}
	// unknown locale.
	if _, err := svc.UpsertNameVariant(ctx, domain.NameVariant{PersonID: p.ID, Locale: "zzz", Name: domain.Name{DisplayName: "x"}}); !errors.Is(err, domain.ErrUnknownLocale) {
		t.Fatalf("unknown locale: want ErrUnknownLocale, got %v", err)
	}
}

// TestResidences adds and replaces a residence row, rejecting an unknown country.
func TestResidences(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)
	p := newPerson(t, svc, "Resident")

	created, err := svc.UpsertResidence(ctx, domain.Residence{PersonID: p.ID, Country: "PL", Region: "Mazowieckie", ValidFrom: "2021-09-01"})
	if err != nil {
		t.Fatalf("add residence: %v", err)
	}
	// replace by id.
	if _, err := svc.UpsertResidence(ctx, domain.Residence{ID: created.ID, PersonID: p.ID, Country: "PL", Region: "Krakow", ValidFrom: "2021-09-01", ValidTo: "2023-01-01"}); err != nil {
		t.Fatalf("replace residence: %v", err)
	}
	rs, _ := svc.ListResidences(ctx, p.ID)
	if len(rs) != 1 || rs[0].Region != "Krakow" || rs[0].ValidTo != "2023-01-01" {
		t.Fatalf("residences = %+v, want one replaced row", rs)
	}
	if _, err := svc.UpsertResidence(ctx, domain.Residence{PersonID: p.ID, Country: "ZZ", ValidFrom: "2020-01-01"}); !errors.Is(err, domain.ErrUnknownCountry) {
		t.Fatalf("unknown country: want ErrUnknownCountry, got %v", err)
	}
}

// TestLifecycleReversible deactivates then reactivates within the grace window.
func TestLifecycleReversible(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)
	p := newPerson(t, svc, "Reversible")

	d, err := svc.DeactivatePerson(ctx, p.ID, "leave")
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if d.Status != domain.StatusDeactivated || d.PurgeAfter == nil {
		t.Fatalf("deactivate state = %+v, want deactivated with purge_after", d)
	}
	r, err := svc.ReactivatePerson(ctx, p.ID)
	if err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if r.Status != domain.StatusActive || r.PurgeAfter != nil {
		t.Fatalf("reactivate state = %+v, want active with no purge_after", r)
	}
	// reactivating an active person is rejected.
	if _, err := svc.ReactivatePerson(ctx, p.ID); !errors.Is(err, domain.ErrLifecycle) {
		t.Fatalf("reactivate active: want ErrLifecycle, got %v", err)
	}
}

// TestPurgeGate refuses purge before the grace window; after it, PII is NULLed and the id remains.
func TestPurgeGate(t *testing.T) {
	ctx := context.Background()

	// Long grace: purge is refused immediately after deactivation.
	svcLong, _ := newService(t, 720)
	refused := newPerson(t, svcLong, "To Be Refused").ID
	if _, err := svcLong.DeactivatePerson(ctx, refused, "x"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svcLong.PurgePerson(ctx, refused); !errors.Is(err, domain.ErrLifecycle) {
		t.Fatalf("purge before grace: want ErrLifecycle, got %v", err)
	}

	// Zero grace: purge is allowed and erases PII.
	svcNow, poolNow := newService(t, 0)
	created, err := svcNow.CreatePerson(ctx, domain.Person{
		Code: code(t, "purge"),
		Name: domain.Name{DisplayName: "Erase Me", Given: "Erase", Surname: "Me"},
		Sex:  "female",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svcNow.UpsertCitizenship(ctx, domain.Citizenship{PersonID: created.ID, Country: "UA", Basis: "birth"}); err != nil {
		t.Fatalf("add citizenship: %v", err)
	}
	if _, err := svcNow.DeactivatePerson(ctx, created.ID, "x"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	purged, err := svcNow.PurgePerson(ctx, created.ID)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged.Status != domain.StatusPurged || purged.DisplayName != "" || purged.Given != "" || purged.Code != "" {
		t.Fatalf("purge did not erase PII: %+v", purged)
	}
	// the id tombstone remains queryable, citizenship rows are gone.
	got, err := svcNow.GetPerson(ctx, created.ID)
	if err != nil {
		t.Fatalf("tombstone get: %v", err)
	}
	if got.Status != domain.StatusPurged || len(got.Citizenships) != 0 {
		t.Fatalf("tombstone = %+v, want purged with no citizenships", got)
	}
	// purge is idempotent.
	if _, err := svcNow.PurgePerson(ctx, created.ID); err != nil {
		t.Fatalf("idempotent purge: %v", err)
	}
	var n int
	if err := poolNow.QueryRow(ctx, "SELECT count(*) FROM oikumenea.person_citizenships WHERE person_id = $1", created.ID).Scan(&n); err != nil {
		t.Fatalf("count citizenships: %v", err)
	}
	if n != 0 {
		t.Fatalf("citizenship rows after purge = %d, want 0", n)
	}
}

// TestContactChannels exercises emails/phones/call signs (D-PersonContactChannels): provider/country
// are derived on write, validation rejects bad input, and a purge erases every channel row.
func TestContactChannels(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t, 0)

	p := newPerson(t, svc, "Contactable Person")

	// Email: provider derived from the domain; primary flag honored.
	email, err := svc.UpsertEmail(ctx, domain.Email{PersonID: p.ID, TypeCode: "personal", Address: "Person@Gmail.com", IsPrimary: true})
	if err != nil {
		t.Fatalf("upsert email: %v", err)
	}
	if email.Address != "person@gmail.com" || email.Provider != "google" || !email.IsPrimary {
		t.Fatalf("email not normalized/derived: %+v", email)
	}
	// Duplicate active address is a conflict.
	if _, err := svc.UpsertEmail(ctx, domain.Email{PersonID: p.ID, TypeCode: "work", Address: "person@gmail.com"}); !errors.Is(err, domain.ErrEmailConflict) {
		t.Fatalf("duplicate email: want ErrEmailConflict, got %v", err)
	}
	// Unknown type code is rejected (FK).
	if _, err := svc.UpsertEmail(ctx, domain.Email{PersonID: p.ID, TypeCode: "nope", Address: "x@y.com"}); !errors.Is(err, domain.ErrUnknownContactType) {
		t.Fatalf("unknown email type: want ErrUnknownContactType, got %v", err)
	}
	// Malformed address is rejected before the DB.
	if _, err := svc.UpsertEmail(ctx, domain.Email{PersonID: p.ID, TypeCode: "personal", Address: "not-an-email"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad email: want ErrInvalid, got %v", err)
	}

	// Phone: E.164-normalized + country derived.
	phone, err := svc.UpsertPhone(ctx, domain.Phone{PersonID: p.ID, TypeCode: "mobile", Number: "+380 (44) 123-45-67"})
	if err != nil {
		t.Fatalf("upsert phone: %v", err)
	}
	if phone.Number != "+380441234567" || phone.Country != "UA" {
		t.Fatalf("phone not normalized/derived: %+v", phone)
	}
	if _, err := svc.UpsertPhone(ctx, domain.Phone{PersonID: p.ID, TypeCode: "mobile", Number: "garbage"}); !errors.Is(err, domain.ErrUnparseablePhone) {
		t.Fatalf("bad phone: want ErrUnparseablePhone, got %v", err)
	}

	// Call sign: required value, unique per person among active.
	if _, err := svc.UpsertCallSign(ctx, domain.CallSign{PersonID: p.ID, CallSign: "Сокіл", IsPrimary: true}); err != nil {
		t.Fatalf("upsert call sign: %v", err)
	}
	if _, err := svc.UpsertCallSign(ctx, domain.CallSign{PersonID: p.ID, CallSign: "Беркут"}); err != nil {
		t.Fatalf("second distinct call sign: %v", err)
	}
	// Duplicate value for the same person is a conflict.
	if _, err := svc.UpsertCallSign(ctx, domain.CallSign{PersonID: p.ID, CallSign: "Сокіл"}); !errors.Is(err, domain.ErrCallSignConflict) {
		t.Fatalf("duplicate call sign: want ErrCallSignConflict, got %v", err)
	}
	// An empty call sign is rejected (NOT NULL).
	if _, err := svc.UpsertCallSign(ctx, domain.CallSign{PersonID: p.ID, CallSign: ""}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("empty call sign: want ErrInvalid, got %v", err)
	}

	// getPerson assembles all three channels.
	got, err := svc.GetPerson(ctx, p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Emails) != 1 || len(got.Phones) != 1 || len(got.CallSigns) != 2 {
		t.Fatalf("channels: emails=%d phones=%d callSigns=%d", len(got.Emails), len(got.Phones), len(got.CallSigns))
	}

	// Purge erases every channel row, keeping the id tombstone.
	if _, err := svc.DeactivatePerson(ctx, p.ID, "x"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svc.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	for _, table := range []string{"person_emails", "person_phones", "person_call_signs"} {
		var n int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM oikumenea."+table+" WHERE person_id = $1", p.ID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("%s rows after purge = %d, want 0", table, n)
		}
	}
}

// TestContactTypeCatalogs reads the seeded email/phone-type catalogs.
func TestContactTypeCatalogs(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)
	ets, err := svc.ListEmailTypes(ctx)
	if err != nil || len(ets) == 0 {
		t.Fatalf("list email types: %d err %v", len(ets), err)
	}
	pts, err := svc.ListPhoneTypes(ctx)
	if err != nil || len(pts) == 0 {
		t.Fatalf("list phone types: %d err %v", len(pts), err)
	}
}

// TestCreateAuditsInOneTx confirms a create records exactly one audit row keyed to it.
func TestCreateAuditsInOneTx(t *testing.T) {
	ctx := context.Background()
	svc, p := newService(t, 720)
	created := newPerson(t, svc, "Audited")

	var n int
	if err := p.QueryRow(ctx,
		"SELECT count(*) FROM oikumenea.audit_log WHERE target_id = $1 AND action = 'person.create' AND actor_type = 'system' AND subsystem = 'person-admin'",
		created.ID,
	).Scan(&n); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if n != 1 {
		t.Fatalf("audit rows for %s = %d, want 1", created.ID, n)
	}
	// audit payload carries no PII (no display name).
	var payload string
	if err := p.QueryRow(ctx,
		"SELECT coalesce(after::text, '') FROM oikumenea.audit_log WHERE target_id = $1 AND action = 'person.create'",
		created.ID,
	).Scan(&payload); err != nil {
		t.Fatalf("query payload: %v", err)
	}
	if want := "Audited"; len(payload) > 0 && contains(payload, want) {
		t.Fatalf("audit payload %q must not contain the display name", payload)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
