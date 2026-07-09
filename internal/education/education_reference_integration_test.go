//go:build integration

// Integration tests for the Education REFERENCE layer (M20 extension / D-Education): curriculum &
// courses, research, governance, credentials, scholarships, and the person↔reference directory links —
// audited CRUD, the prerequisite cycle guard, and person-purge erasure of every person-binding table.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/education/...
package education_test

import (
	"context"
	"errors"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/education/domain"
	personadapters "github.com/olegamysk/go-oikumenea/internal/person/adapters"
)

// TestEducationReferenceVertical drives the reference-layer slice in one ordered scenario.
func TestEducationReferenceVertical(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	uniKind := catalogID(t, pool, "education_institution_kinds", "university")
	masterLevel := catalogID(t, pool, "education_degree_levels", "isced-7")

	inst, err := svc.CreateInstitution(ctx, domain.InstitutionInput{Code: uniq("ref-uni"), Name: "Ref University", KindID: uniKind})
	if err != nil {
		t.Fatalf("create institution: %v", err)
	}

	// --- program + curriculum version + courses + curriculum item ---
	prog, err := svc.CreateProgram(ctx, inst.ID, domain.ProgramInput{Code: uniq("se-msc"), Name: "MSc Software Engineering", DegreeLevelID: ptr(masterLevel), DurationYears: ptr("1.5"), CreditHoursTotal: intp(90)})
	if err != nil {
		t.Fatalf("create program: %v", err)
	}
	assertOneAction(t, pool, prog.ID, "education.program.create")
	if prog.DurationYears != "1.5" {
		t.Fatalf("expected durationYears round-trip 1.5, got %q", prog.DurationYears)
	}

	ver, err := svc.CreateCurriculumVersion(ctx, prog.ID, domain.CurriculumVersionInput{VersionCode: "2024-v1", Status: ptr("active")})
	if err != nil {
		t.Fatalf("create curriculum version: %v", err)
	}
	courseA, err := svc.CreateCourse(ctx, inst.ID, domain.CourseInput{Code: uniq("cs101"), Title: "Intro to CS", CreditHours: intp(5), Level: intp(100)})
	if err != nil {
		t.Fatalf("create course A: %v", err)
	}
	courseB, err := svc.CreateCourse(ctx, inst.ID, domain.CourseInput{Code: uniq("cs201"), Title: "Algorithms", CreditHours: intp(6), Level: intp(200)})
	if err != nil {
		t.Fatalf("create course B: %v", err)
	}
	if _, err := svc.AddCurriculumItem(ctx, ver.ID, domain.CurriculumItemInput{CourseID: courseB.ID, YearOfStudy: intp(1)}); err != nil {
		t.Fatalf("add curriculum item: %v", err)
	}
	items, err := svc.ListCurriculumItems(ctx, ver.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("expected 1 curriculum item, got %d (err %v)", len(items), err)
	}

	// --- prerequisites + cycle guard: B requires A; then A requires B must be rejected ---
	if _, err := svc.AddCoursePrerequisite(ctx, courseB.ID, domain.CoursePrerequisiteInput{RequiredCourseID: courseA.ID}); err != nil {
		t.Fatalf("add prerequisite B->A: %v", err)
	}
	if _, err := svc.AddCoursePrerequisite(ctx, courseA.ID, domain.CoursePrerequisiteInput{RequiredCourseID: courseB.ID}); !errors.Is(err, domain.ErrPrereqCycle) {
		t.Fatalf("expected ErrPrereqCycle for A->B, got %v", err)
	}

	// --- research centre + group; grant; governance body; scholarship; publication; qualification ---
	centre, err := svc.CreateResearchCentre(ctx, inst.ID, domain.ResearchCentreInput{Code: uniq("ai-lab"), Name: "AI Lab", Kind: ptr("lab")})
	if err != nil {
		t.Fatalf("create research centre: %v", err)
	}
	rg, err := svc.CreateResearchGroup(ctx, inst.ID, domain.ResearchGroupInput{Code: uniq("nlp"), Name: "NLP Group", CentreID: ptr(centre.ID)})
	if err != nil {
		t.Fatalf("create research group: %v", err)
	}
	grant, err := svc.CreateGrant(ctx, inst.ID, domain.GrantInput{Code: uniq("grant-1"), Title: "NLP Grant", Amount: ptr("250000.00"), Currency: ptr("EUR"), Status: ptr("active")})
	if err != nil {
		t.Fatalf("create grant: %v", err)
	}
	if grant.Amount != "250000.00" {
		t.Fatalf("expected grant amount round-trip 250000.00, got %q", grant.Amount)
	}
	body, err := svc.CreateGovernanceBody(ctx, inst.ID, domain.GovernanceBodyInput{Code: uniq("senate"), Name: "Academic Senate", Kind: ptr("senate")})
	if err != nil {
		t.Fatalf("create governance body: %v", err)
	}
	if _, err := svc.CreatePolicy(ctx, inst.ID, domain.PolicyInput{Code: uniq("acad-pol"), Title: "Academic Integrity", GovernanceBodyID: ptr(body.ID), Status: ptr("active")}); err != nil {
		t.Fatalf("create policy: %v", err)
	}
	sch, err := svc.CreateScholarship(ctx, domain.ScholarshipInput{Code: uniq("merit-1"), Name: "Merit Scholarship", Kind: ptr("merit"), Amount: ptr("1200.00"), Currency: ptr("USD")})
	if err != nil {
		t.Fatalf("create scholarship: %v", err)
	}
	pub, err := svc.CreatePublication(ctx, domain.PublicationInput{Code: uniq("pub-1"), Title: "On Transformers", Kind: ptr("journal_article"), InstitutionID: ptr(inst.ID), OpenAccess: boolp(true)})
	if err != nil {
		t.Fatalf("create publication: %v", err)
	}
	qual, err := svc.CreateQualification(ctx, inst.ID, domain.QualificationInput{Code: uniq("msc-se"), Name: "MSc Software Engineering", ProgramID: ptr(prog.ID), DegreeLevelID: ptr(masterLevel), FrameworkCode: ptr("ISCED")})
	if err != nil {
		t.Fatalf("create qualification: %v", err)
	}

	// accreditation event against the program.
	if _, err := svc.CreateAccreditationEvent(ctx, domain.AccreditationEventInput{EntityKind: "program", ProgramID: ptr(prog.ID), Outcome: ptr("granted")}); err != nil {
		t.Fatalf("create accreditation event: %v", err)
	}

	// --- person: enrollment linked to the program + student number, then all six person links ---
	person := seedPerson(t, pool)
	enr, err := svc.CreateEnrollment(ctx, person, domain.EnrollmentInput{InstitutionID: inst.ID, ProgramID: ptr(prog.ID), StudentNumber: ptr("SN-2024-001"), DegreeLevelID: ptr(masterLevel)})
	if err != nil {
		t.Fatalf("create enrollment: %v", err)
	}
	if enr.ProgramID != prog.ID || enr.StudentNumber != "SN-2024-001" {
		t.Fatalf("expected enrollment program/student-number set, got %q / %q", enr.ProgramID, enr.StudentNumber)
	}

	auth, err := svc.CreatePublicationAuthorship(ctx, person, domain.PublicationAuthorshipInput{PublicationID: pub.ID, AuthorOrder: intp(1), Corresponding: boolp(true)})
	if err != nil {
		t.Fatalf("create authorship: %v", err)
	}
	assertOneAction(t, pool, auth.ID, "education.publication-authorship.create")
	if _, err := svc.CreateResearchMembership(ctx, person, domain.ResearchMembershipInput{GroupID: rg.ID, Role: ptr("lead")}); err != nil {
		t.Fatalf("create research membership: %v", err)
	}
	if _, err := svc.CreateGrantHolding(ctx, person, domain.GrantHoldingInput{GrantID: grant.ID, Role: ptr("pi")}); err != nil {
		t.Fatalf("create grant holding: %v", err)
	}
	if _, err := svc.CreateGovernanceMembership(ctx, person, domain.GovernanceMembershipInput{BodyID: body.ID, RoleInBody: ptr("Chair")}); err != nil {
		t.Fatalf("create governance membership: %v", err)
	}
	award, err := svc.CreateQualificationAward(ctx, person, domain.QualificationAwardInput{QualificationID: qual.ID, EnrollmentID: ptr(enr.ID), WithDistinction: boolp(true), Gpa: ptr("3.950")})
	if err != nil {
		t.Fatalf("create qualification award: %v", err)
	}
	if award.Gpa != "3.950" {
		t.Fatalf("expected gpa round-trip 3.950, got %q", award.Gpa)
	}
	if _, err := svc.CreateScholarshipAward(ctx, person, domain.ScholarshipAwardInput{ScholarshipID: sch.ID}); err != nil {
		t.Fatalf("create scholarship award: %v", err)
	}

	// --- purge erases every person-binding link; reference entities remain ---
	if _, err := personadapters.NewRepository(pool).Purge(ctx, person); err != nil {
		t.Fatalf("purge person: %v", err)
	}
	firePersonPurge(t, ctx, pool, svc, person) // education erases its own rows via PersonPurged (D-PersonModuleSplit)
	if rows, _ := svc.ListPublicationAuthorships(ctx, person); len(rows) != 0 {
		t.Fatalf("expected authorships erased, got %d", len(rows))
	}
	if rows, _ := svc.ListResearchMemberships(ctx, person); len(rows) != 0 {
		t.Fatalf("expected research memberships erased, got %d", len(rows))
	}
	if rows, _ := svc.ListGrantHoldings(ctx, person); len(rows) != 0 {
		t.Fatalf("expected grant holdings erased, got %d", len(rows))
	}
	if rows, _ := svc.ListGovernanceMemberships(ctx, person); len(rows) != 0 {
		t.Fatalf("expected governance memberships erased, got %d", len(rows))
	}
	if rows, _ := svc.ListQualificationAwards(ctx, person); len(rows) != 0 {
		t.Fatalf("expected qualification awards erased, got %d", len(rows))
	}
	if rows, _ := svc.ListScholarshipAwards(ctx, person); len(rows) != 0 {
		t.Fatalf("expected scholarship awards erased, got %d", len(rows))
	}
	// reference entities survive the purge.
	if _, err := svc.GetPublication(ctx, pub.ID); err != nil {
		t.Fatalf("publication should survive purge: %v", err)
	}
	if _, err := svc.GetGrant(ctx, grant.ID); err != nil {
		t.Fatalf("grant should survive purge: %v", err)
	}
}

func intp(i int) *int    { return &i }
func boolp(b bool) *bool { return &b }
