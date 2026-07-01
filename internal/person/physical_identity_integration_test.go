//go:build integration

// Integration tests for the M31 physical-identity slice (D-PhysicalIdentity / D-SpecialPII) against a
// real Postgres. Proves the exit criteria:
//
//   - record an AKA + a former-legal name as name variants (aliases folded into person_name_variants);
//
//   - a physical description with a blood type + a distinguishing mark;
//
//   - a declared ethnicity stored ENCRYPTED with a legal_basis (ciphertext at rest holds no plaintext,
//     a blind index is present, decrypt round-trips);
//
//   - purge crypto-erases the ethnicity (envelope dropped, row tombstone) and hard-deletes the
//     description/mark.
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/person/...
package person_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
)

// colorID resolves a seeded platform_colors RID by palette domain + code (D-Color).
func colorID(t *testing.T, pool *pgxpool.Pool, domainName, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM oikumenea.platform_colors WHERE domain = $1 AND code = $2 AND deleted_at IS NULL`, domainName, code).Scan(&id); err != nil {
		t.Fatalf("resolve color %s/%s: %v", domainName, code, err)
	}
	return id
}

// seedEthnicityType inserts a declared-ethnicity catalog row directly (the open, operator-managed
// vocabulary is seeded empty by the migration) and returns its code.
func seedEthnicityType(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	c := "eth-" + uuid.NewString()[:8]
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO oikumenea.person_ethnicity_types (code, name) VALUES ($1,$2)`, c, name); err != nil {
		t.Fatalf("seed ethnicity type: %v", err)
	}
	return c
}

func TestNameAliases(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)
	p := newPerson(t, svc, "Ivan Petrenko")

	aka, err := svc.AddNameAlias(ctx, domain.NameVariant{
		PersonID:    p.ID,
		Locale:      "eng",
		Name:        domain.Name{DisplayName: "The Fox"},
		VariantKind: domain.VariantAKA,
		Source:      "operator_verified",
		Confidence:  "probable",
	})
	if err != nil {
		t.Fatalf("add aka: %v", err)
	}
	former, err := svc.AddNameAlias(ctx, domain.NameVariant{
		PersonID:    p.ID,
		Locale:      "ukr",
		Name:        domain.Name{DisplayName: "Іван Сидоренко"},
		VariantKind: domain.VariantFormerLegal,
	})
	if err != nil {
		t.Fatalf("add former_legal: %v", err)
	}

	// An invalid alias kind (e.g. the transliteration kind) is rejected.
	if _, err := svc.AddNameAlias(ctx, domain.NameVariant{
		PersonID: p.ID, Locale: "eng", Name: domain.Name{DisplayName: "x"}, VariantKind: domain.VariantTransliteration,
	}); err == nil {
		t.Fatal("expected an invalid-alias-kind error for variantKind=transliteration")
	}

	// A canonical transliteration coexists with the aliases under the same locale (partial uniqueness).
	if _, err := svc.UpsertNameVariant(ctx, domain.NameVariant{
		PersonID: p.ID, Locale: "eng", Name: domain.Name{DisplayName: "Ivan Petrenko"},
	}); err != nil {
		t.Fatalf("upsert transliteration: %v", err)
	}

	vars, err := svc.ListNameVariants(ctx, p.ID)
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	var akas, formers, translits int
	for _, v := range vars {
		switch v.VariantKind {
		case domain.VariantAKA:
			akas++
		case domain.VariantFormerLegal:
			formers++
		case domain.VariantTransliteration:
			translits++
		}
	}
	if akas != 1 || formers != 1 || translits != 1 {
		t.Fatalf("expected 1 aka + 1 former_legal + 1 transliteration, got aka=%d former=%d translit=%d", akas, formers, translits)
	}

	// Delete the AKA by its RID; the former-legal alias remains.
	if err := svc.DeleteNameAlias(ctx, p.ID, aka.ID); err != nil {
		t.Fatalf("delete alias: %v", err)
	}
	vars, _ = svc.ListNameVariants(ctx, p.ID)
	for _, v := range vars {
		if v.ID == aka.ID {
			t.Fatal("deleted AKA still present")
		}
	}
	if former.ID == "" {
		t.Fatal("former-legal alias missing an id")
	}
}

// TestEthnicityTypeHierarchy proves the person read surface over a hierarchical catalog (D-PhysicalIdentity
// amendment, M43): roots vs children filters, HasChildren, and getEthnicityType assembling the group's
// associated-language + homeland-country RIDs. The catalog rows + links are seeded directly (the import
// pipeline is covered in the dataimport suite).
func TestEthnicityTypeHierarchy(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t, 720)

	// Seed a synthetic languoid + resolve a seeded country to link the child group to.
	var langID, uaID string
	if err := pool.QueryRow(ctx, `INSERT INTO oikumenea.language_languoids (code, level, name)
		VALUES ('zzethl01', 'language', 'Zz Eth Lang') RETURNING id`).Scan(&langID); err != nil {
		t.Fatalf("seed languoid: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM oikumenea.geo_countries WHERE code = 'UA'`).Scan(&uaID); err != nil {
		t.Fatalf("resolve UA: %v", err)
	}
	var rootID, childID string
	if err := pool.QueryRow(ctx, `INSERT INTO oikumenea.person_ethnicity_types (code, name) VALUES ('zzeth-root','Zz Root') RETURNING id`).Scan(&rootID); err != nil {
		t.Fatalf("seed root: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO oikumenea.person_ethnicity_types (code, name, parent_id) VALUES ('zzeth-kid','Zz Kid',$1) RETURNING id`, rootID).Scan(&childID); err != nil {
		t.Fatalf("seed child: %v", err)
	}
	pool.Exec(ctx, `INSERT INTO oikumenea.person_ethnicity_type_languages (ethnicity_type_id, language_id) VALUES ($1,$2)`, childID, langID)
	pool.Exec(ctx, `INSERT INTO oikumenea.person_ethnicity_type_countries (ethnicity_type_id, country_id) VALUES ($1,$2)`, childID, uaID)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM oikumenea.person_ethnicity_type_languages WHERE ethnicity_type_id = $1`, childID)
		pool.Exec(ctx, `DELETE FROM oikumenea.person_ethnicity_type_countries WHERE ethnicity_type_id = $1`, childID)
		pool.Exec(ctx, `DELETE FROM oikumenea.person_ethnicity_types WHERE code IN ('zzeth-kid','zzeth-root')`)
		pool.Exec(ctx, `DELETE FROM oikumenea.language_languoids WHERE code = 'zzethl01'`)
	})

	// Roots filter: the root appears with HasChildren; the child does not.
	roots, err := svc.ListEthnicityTypes(ctx, domain.EthnicityTypeFilter{TopLevel: true})
	if err != nil {
		t.Fatalf("list roots: %v", err)
	}
	var sawRoot bool
	for _, e := range roots {
		if e.ID == rootID {
			sawRoot = true
			if !e.HasChildren {
				t.Fatal("root should report HasChildren=true")
			}
		}
		if e.ID == childID {
			t.Fatal("child must not appear in the roots filter")
		}
	}
	if !sawRoot {
		t.Fatal("root missing from roots filter")
	}

	// Children filter: only the child.
	kids, err := svc.ListEthnicityTypes(ctx, domain.EthnicityTypeFilter{Parent: rootID})
	if err != nil || len(kids) != 1 || kids[0].ID != childID {
		t.Fatalf("children filter mismatch: %+v err=%v", kids, err)
	}

	// getEthnicityType: parent + the group-level language + country RIDs.
	et, langs, countries, err := svc.GetEthnicityType(ctx, childID)
	if err != nil {
		t.Fatalf("get ethnicity type: %v", err)
	}
	if et.ParentID != rootID {
		t.Fatalf("child parent = %q, want %q", et.ParentID, rootID)
	}
	if len(langs) != 1 || langs[0] != langID {
		t.Fatalf("languages = %v, want [%s]", langs, langID)
	}
	if len(countries) != 1 || countries[0] != uaID {
		t.Fatalf("countries = %v, want [%s]", countries, uaID)
	}
}

func TestPhysicalDescriptionAndMarks(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t, 720)
	p := newPerson(t, svc, "Olena Koval")

	h, w := 172, 64
	eyeBrown := colorID(t, pool, "eye", "brown")
	hairBlack := colorID(t, pool, "hair", "black")
	desc, err := svc.UpsertPhysicalDescription(ctx, domain.PhysicalDescription{
		PersonID: p.ID, HeightCm: &h, WeightKg: &w, EyeColorID: eyeBrown, HairColorID: hairBlack,
		Build: "athletic", BloodType: "O+", EffectiveFrom: "2026-01-01",
	})
	if err != nil {
		t.Fatalf("upsert physical description: %v", err)
	}
	if desc.BloodType != "O+" || desc.HeightCm == nil || *desc.HeightCm != 172 {
		t.Fatalf("description round-trip mismatch: %+v", desc)
	}
	if desc.EyeColorID != eyeBrown || desc.HairColorID != hairBlack {
		t.Fatalf("color round-trip mismatch: %+v", desc)
	}

	// A color from the wrong palette is rejected by the hard-FK domain check (D-Color): a vehicle
	// color is not a valid eye color.
	vehBlue := colorID(t, pool, "vehicle", "blue")
	if _, err := svc.UpsertPhysicalDescription(ctx, domain.PhysicalDescription{PersonID: p.ID, EyeColorID: vehBlue}); !errors.Is(err, domain.ErrColorMismatch) {
		t.Fatalf("expected ErrColorMismatch for a vehicle-palette eye color, got %v", err)
	}

	// An out-of-range height is rejected by the domain validator.
	bad := 999
	if _, err := svc.UpsertPhysicalDescription(ctx, domain.PhysicalDescription{PersonID: p.ID, HeightCm: &bad}); err == nil {
		t.Fatal("expected invalid-height error")
	}

	mark, err := svc.UpsertDistinguishingMark(ctx, domain.DistinguishingMark{
		PersonID: p.ID, Kind: "tattoo", BodyLocation: "left forearm", Description: "anchor",
	})
	if err != nil {
		t.Fatalf("upsert mark: %v", err)
	}
	if _, err := svc.UpsertDistinguishingMark(ctx, domain.DistinguishingMark{PersonID: p.ID, Kind: "bogus"}); err == nil {
		t.Fatal("expected invalid-kind error")
	}

	marks, err := svc.ListDistinguishingMarks(ctx, p.ID)
	if err != nil || len(marks) != 1 || marks[0].ID != mark.ID {
		t.Fatalf("list marks mismatch: %+v err=%v", marks, err)
	}
	descs, err := svc.ListPhysicalDescriptions(ctx, p.ID)
	if err != nil || len(descs) != 1 {
		t.Fatalf("list descriptions mismatch: %+v err=%v", descs, err)
	}
}

func TestDeclaredEthnicityEncrypted(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t, 720)
	p := newPerson(t, svc, "Bohdan Tkachenko")

	code := seedEthnicityType(t, pool, "Ukrainian")

	// An unknown code is rejected (catalog-typed).
	if _, err := svc.AddEthnicity(ctx, p.ID, "no-such-code", "explicit_consent", "", ""); err == nil {
		t.Fatal("expected unknown-ethnicity-type error")
	}
	// An unknown legal basis is rejected by the FK.
	if _, err := svc.AddEthnicity(ctx, p.ID, code, "no-such-basis", "", ""); err == nil {
		t.Fatal("expected unknown-legal-basis error")
	}

	eth, err := svc.AddEthnicity(ctx, p.ID, code, "explicit_consent", "self_declared", "confirmed")
	if err != nil {
		t.Fatalf("add ethnicity: %v", err)
	}
	if eth.Code != code || eth.Name != "Ukrainian" || eth.LegalBasis != "explicit_consent" {
		t.Fatalf("ethnicity round-trip mismatch: %+v", eth)
	}

	// Ciphertext at rest holds NO plaintext, and a blind index IS present.
	var ct, blind []byte
	if err := pool.QueryRow(ctx,
		`SELECT value_ciphertext, value_blind_index FROM oikumenea.person_ethnicities WHERE id = $1`, eth.ID).
		Scan(&ct, &blind); err != nil {
		t.Fatalf("read raw ethnicity: %v", err)
	}
	if len(ct) == 0 || contains(string(ct), code) {
		t.Fatalf("expected non-empty ciphertext without plaintext, got %q", string(ct))
	}
	if len(blind) == 0 {
		t.Fatal("expected a blind index at rest")
	}

	// Decrypt round-trips through the list path.
	list, err := svc.ListEthnicities(ctx, p.ID)
	if err != nil || len(list) != 1 || list[0].Code != code {
		t.Fatalf("list ethnicities mismatch: %+v err=%v", list, err)
	}

	// Purge crypto-erases the ethnicity (row survives, envelope dropped) and removes description/mark.
	if _, err := svc.UpsertPhysicalDescription(ctx, domain.PhysicalDescription{PersonID: p.ID, BloodType: "A+"}); err != nil {
		t.Fatalf("seed description: %v", err)
	}
	if _, err := svc.UpsertDistinguishingMark(ctx, domain.DistinguishingMark{PersonID: p.ID, Kind: "scar"}); err != nil {
		t.Fatalf("seed mark: %v", err)
	}
	// A zero-grace service deactivates+purges immediately; it shares the same pool/DB.
	svcNow, _ := newService(t, 0)
	if _, err := svcNow.DeactivatePerson(ctx, p.ID, "x"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svcNow.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}

	// Description + mark are gone; the ethnicity row survives as a crypto-erased tombstone.
	var nDesc, nMark, nEth, nEnvelope int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM oikumenea.person_physical_descriptions WHERE person_id=$1),
		(SELECT count(*) FROM oikumenea.person_distinguishing_marks WHERE person_id=$1),
		(SELECT count(*) FROM oikumenea.person_ethnicities WHERE person_id=$1),
		(SELECT count(*) FROM oikumenea.person_ethnicities WHERE person_id=$1 AND value_ciphertext IS NOT NULL)`,
		p.ID).Scan(&nDesc, &nMark, &nEth, &nEnvelope); err != nil {
		t.Fatalf("post-purge counts: %v", err)
	}
	if nDesc != 0 || nMark != 0 {
		t.Fatalf("expected description+mark hard-deleted, got desc=%d mark=%d", nDesc, nMark)
	}
	if nEth != 1 || nEnvelope != 0 {
		t.Fatalf("expected the ethnicity row kept as a crypto-erased tombstone, got rows=%d withEnvelope=%d", nEth, nEnvelope)
	}
}
