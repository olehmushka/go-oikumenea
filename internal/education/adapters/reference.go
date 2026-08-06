// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/education/adapters/educationsql"
	"github.com/olehmushka/go-oikumenea/internal/education/domain"
)

// Reference-layer persistence (M20 extension). Mirrors repository.go: domain values ↔ educationsql
// params/rows, with FK/unique violations mapped to domain sentinels via mapErr/notFound.

// ---------------------------------------------------------------- programs

func (r *Repository) InsertProgram(ctx context.Context, institutionID string, in domain.ProgramInput) (domain.Program, error) {
	row, err := r.q.InsertProgram(ctx, educationsql.InsertProgramParams{
		InstitutionID: institutionID, OwningUnitID: text(in.OwningUnitID), DegreeLevelID: text(in.DegreeLevelID),
		Code: in.Code, Name: in.Name, Mode: iface(in.Mode), DurationYears: numArg(in.DurationYears), CreditHoursTotal: int4(in.CreditHoursTotal),
	})
	if err != nil {
		return domain.Program{}, mapErr(err)
	}
	return toProgram(row), nil
}

func (r *Repository) GetProgram(ctx context.Context, id string) (domain.Program, error) {
	row, err := r.q.GetProgram(ctx, id)
	if err != nil {
		return domain.Program{}, notFound(err, domain.ErrRefNotFound)
	}
	return toProgram(row), nil
}

func (r *Repository) UpdateProgram(ctx context.Context, id string, in domain.ProgramInput) (domain.Program, error) {
	row, err := r.q.UpdateProgram(ctx, educationsql.UpdateProgramParams{
		Name: ptrText(in.Name), OwningUnitID: text(in.OwningUnitID), DegreeLevelID: text(in.DegreeLevelID),
		Mode: text(in.Mode), DurationYears: numArg(in.DurationYears), CreditHoursTotal: int4(in.CreditHoursTotal), State: text(in.State), ID: id,
	})
	if err != nil {
		return domain.Program{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toProgram(row), nil
}

func (r *Repository) ListProgramsByInstitution(ctx context.Context, institutionID string) ([]domain.Program, error) {
	rows, err := r.q.ListProgramsByInstitution(ctx, institutionID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Program, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProgram(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteProgram(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteProgram(ctx, id)
}

// ---------------------------------------------------------------- courses

func (r *Repository) InsertCourse(ctx context.Context, institutionID string, in domain.CourseInput) (domain.Course, error) {
	row, err := r.q.InsertCourse(ctx, educationsql.InsertCourseParams{
		InstitutionID: institutionID, OwningUnitID: text(in.OwningUnitID), Code: in.Code, Title: in.Title,
		CreditHours: int4(in.CreditHours), Level: int4(in.Level), Description: text(in.Description), DeliveryMode: iface(in.DeliveryMode),
	})
	if err != nil {
		return domain.Course{}, mapErr(err)
	}
	return toCourse(row), nil
}

func (r *Repository) GetCourse(ctx context.Context, id string) (domain.Course, error) {
	row, err := r.q.GetCourse(ctx, id)
	if err != nil {
		return domain.Course{}, notFound(err, domain.ErrRefNotFound)
	}
	return toCourse(row), nil
}

func (r *Repository) UpdateCourse(ctx context.Context, id string, in domain.CourseInput) (domain.Course, error) {
	row, err := r.q.UpdateCourse(ctx, educationsql.UpdateCourseParams{
		Title: ptrText(in.Title), OwningUnitID: text(in.OwningUnitID), CreditHours: int4(in.CreditHours), Level: int4(in.Level),
		Description: text(in.Description), DeliveryMode: text(in.DeliveryMode), Status: text(in.Status), ID: id,
	})
	if err != nil {
		return domain.Course{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toCourse(row), nil
}

func (r *Repository) ListCoursesByInstitution(ctx context.Context, institutionID string) ([]domain.Course, error) {
	rows, err := r.q.ListCoursesByInstitution(ctx, institutionID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Course, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCourse(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteCourse(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteCourse(ctx, id)
}

// ---------------------------------------------------------------- curriculum versions

func (r *Repository) InsertCurriculumVersion(ctx context.Context, programID string, in domain.CurriculumVersionInput) (domain.CurriculumVersion, error) {
	row, err := r.q.InsertCurriculumVersion(ctx, educationsql.InsertCurriculumVersionParams{
		ProgramID: programID, VersionCode: in.VersionCode, EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo), Status: iface(in.Status),
	})
	if err != nil {
		return domain.CurriculumVersion{}, mapErr(err)
	}
	return toCurriculumVersion(row), nil
}

func (r *Repository) GetCurriculumVersion(ctx context.Context, id string) (domain.CurriculumVersion, error) {
	row, err := r.q.GetCurriculumVersion(ctx, id)
	if err != nil {
		return domain.CurriculumVersion{}, notFound(err, domain.ErrRefNotFound)
	}
	return toCurriculumVersion(row), nil
}

func (r *Repository) UpdateCurriculumVersion(ctx context.Context, id string, in domain.CurriculumVersionInput) (domain.CurriculumVersion, error) {
	row, err := r.q.UpdateCurriculumVersion(ctx, educationsql.UpdateCurriculumVersionParams{
		VersionCode: ptrText(in.VersionCode), EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo), Status: text(in.Status), ID: id,
	})
	if err != nil {
		return domain.CurriculumVersion{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toCurriculumVersion(row), nil
}

func (r *Repository) ListCurriculumVersionsByProgram(ctx context.Context, programID string) ([]domain.CurriculumVersion, error) {
	rows, err := r.q.ListCurriculumVersionsByProgram(ctx, programID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CurriculumVersion, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCurriculumVersion(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteCurriculumVersion(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteCurriculumVersion(ctx, id)
}

// ---------------------------------------------------------------- curriculum items

func (r *Repository) InsertCurriculumItem(ctx context.Context, versionID string, in domain.CurriculumItemInput) (domain.CurriculumItem, error) {
	row, err := r.q.InsertCurriculumItem(ctx, educationsql.InsertCurriculumItemParams{
		VersionID: versionID, CourseID: in.CourseID, IsRequired: bool4(in.IsRequired),
		YearOfStudy: int4(in.YearOfStudy), CreditAllocation: int4(in.CreditAllocation), SemesterSlot: int4(in.SemesterSlot),
	})
	if err != nil {
		return domain.CurriculumItem{}, mapErr(err)
	}
	return toCurriculumItem(row), nil
}

func (r *Repository) GetCurriculumItem(ctx context.Context, id string) (domain.CurriculumItem, error) {
	row, err := r.q.GetCurriculumItem(ctx, id)
	if err != nil {
		return domain.CurriculumItem{}, notFound(err, domain.ErrRefNotFound)
	}
	return toCurriculumItem(row), nil
}

func (r *Repository) UpdateCurriculumItem(ctx context.Context, id string, in domain.CurriculumItemInput) (domain.CurriculumItem, error) {
	row, err := r.q.UpdateCurriculumItem(ctx, educationsql.UpdateCurriculumItemParams{
		IsRequired: bool4(in.IsRequired), YearOfStudy: int4(in.YearOfStudy), CreditAllocation: int4(in.CreditAllocation), SemesterSlot: int4(in.SemesterSlot), ID: id,
	})
	if err != nil {
		return domain.CurriculumItem{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toCurriculumItem(row), nil
}

func (r *Repository) ListCurriculumItemsByVersion(ctx context.Context, versionID string) ([]domain.CurriculumItem, error) {
	rows, err := r.q.ListCurriculumItemsByVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CurriculumItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCurriculumItem(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteCurriculumItem(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteCurriculumItem(ctx, id)
}

// ---------------------------------------------------------------- course prerequisites

func (r *Repository) InsertCoursePrerequisite(ctx context.Context, courseID string, in domain.CoursePrerequisiteInput) (domain.CoursePrerequisite, error) {
	row, err := r.q.InsertCoursePrerequisite(ctx, educationsql.InsertCoursePrerequisiteParams{
		CourseID: courseID, RequiredCourseID: in.RequiredCourseID, Kind: iface(in.Kind), MinGrade: text(in.MinGrade),
	})
	if err != nil {
		return domain.CoursePrerequisite{}, mapErr(err)
	}
	return toCoursePrerequisite(row), nil
}

func (r *Repository) GetCoursePrerequisite(ctx context.Context, id string) (domain.CoursePrerequisite, error) {
	row, err := r.q.GetCoursePrerequisite(ctx, id)
	if err != nil {
		return domain.CoursePrerequisite{}, notFound(err, domain.ErrRefNotFound)
	}
	return toCoursePrerequisite(row), nil
}

func (r *Repository) ListCoursePrerequisitesByCourse(ctx context.Context, courseID string) ([]domain.CoursePrerequisite, error) {
	rows, err := r.q.ListCoursePrerequisitesByCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CoursePrerequisite, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCoursePrerequisite(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteCoursePrerequisite(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteCoursePrerequisite(ctx, id)
}

func (r *Repository) ListPrerequisiteEdges(ctx context.Context) ([]domain.PrereqEdge, error) {
	rows, err := r.q.ListPrerequisiteEdges(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PrereqEdge, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.PrereqEdge{CourseID: row.CourseID, RequiredCourseID: row.RequiredCourseID})
	}
	return out, nil
}

// ---------------------------------------------------------------- research centres

func (r *Repository) InsertResearchCentre(ctx context.Context, institutionID string, in domain.ResearchCentreInput) (domain.ResearchCentre, error) {
	row, err := r.q.InsertResearchCentre(ctx, educationsql.InsertResearchCentreParams{
		InstitutionID: institutionID, Code: in.Code, Name: in.Name, Kind: iface(in.Kind),
		FundingSource: text(in.FundingSource), FoundedOn: datePtr(in.FoundedOn), DissolvedOn: datePtr(in.DissolvedOn),
	})
	if err != nil {
		return domain.ResearchCentre{}, mapErr(err)
	}
	return toResearchCentre(row), nil
}

func (r *Repository) GetResearchCentre(ctx context.Context, id string) (domain.ResearchCentre, error) {
	row, err := r.q.GetResearchCentre(ctx, id)
	if err != nil {
		return domain.ResearchCentre{}, notFound(err, domain.ErrRefNotFound)
	}
	return toResearchCentre(row), nil
}

func (r *Repository) UpdateResearchCentre(ctx context.Context, id string, in domain.ResearchCentreInput) (domain.ResearchCentre, error) {
	row, err := r.q.UpdateResearchCentre(ctx, educationsql.UpdateResearchCentreParams{
		Name: ptrText(in.Name), Kind: text(in.Kind), FundingSource: text(in.FundingSource),
		FoundedOn: datePtr(in.FoundedOn), DissolvedOn: datePtr(in.DissolvedOn), Status: text(in.Status), ID: id,
	})
	if err != nil {
		return domain.ResearchCentre{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toResearchCentre(row), nil
}

func (r *Repository) ListResearchCentresByInstitution(ctx context.Context, institutionID string) ([]domain.ResearchCentre, error) {
	rows, err := r.q.ListResearchCentresByInstitution(ctx, institutionID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ResearchCentre, 0, len(rows))
	for _, row := range rows {
		out = append(out, toResearchCentre(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteResearchCentre(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteResearchCentre(ctx, id)
}

// ---------------------------------------------------------------- research groups

func (r *Repository) InsertResearchGroup(ctx context.Context, institutionID string, in domain.ResearchGroupInput) (domain.ResearchGroup, error) {
	row, err := r.q.InsertResearchGroup(ctx, educationsql.InsertResearchGroupParams{
		InstitutionID: institutionID, CentreID: text(in.CentreID), UnitID: text(in.UnitID), Code: in.Code, Name: in.Name, FocusArea: text(in.FocusArea),
	})
	if err != nil {
		return domain.ResearchGroup{}, mapErr(err)
	}
	return toResearchGroup(row), nil
}

func (r *Repository) GetResearchGroup(ctx context.Context, id string) (domain.ResearchGroup, error) {
	row, err := r.q.GetResearchGroup(ctx, id)
	if err != nil {
		return domain.ResearchGroup{}, notFound(err, domain.ErrRefNotFound)
	}
	return toResearchGroup(row), nil
}

func (r *Repository) UpdateResearchGroup(ctx context.Context, id string, in domain.ResearchGroupInput) (domain.ResearchGroup, error) {
	row, err := r.q.UpdateResearchGroup(ctx, educationsql.UpdateResearchGroupParams{
		Name: ptrText(in.Name), CentreID: text(in.CentreID), UnitID: text(in.UnitID), FocusArea: text(in.FocusArea), Status: text(in.Status), ID: id,
	})
	if err != nil {
		return domain.ResearchGroup{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toResearchGroup(row), nil
}

func (r *Repository) ListResearchGroupsByInstitution(ctx context.Context, institutionID string) ([]domain.ResearchGroup, error) {
	rows, err := r.q.ListResearchGroupsByInstitution(ctx, institutionID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ResearchGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, toResearchGroup(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteResearchGroup(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteResearchGroup(ctx, id)
}

// ---------------------------------------------------------------- grants

func (r *Repository) InsertGrant(ctx context.Context, institutionID string, in domain.GrantInput) (domain.Grant, error) {
	row, err := r.q.InsertGrant(ctx, educationsql.InsertGrantParams{
		InstitutionID: institutionID, Code: in.Code, Title: in.Title, Funder: text(in.Funder), FunderRef: text(in.FunderRef),
		Amount: numArg(in.Amount), Currency: text(in.Currency), StartOn: datePtr(in.StartOn), EndOn: datePtr(in.EndOn), Status: iface(in.Status),
	})
	if err != nil {
		return domain.Grant{}, mapErr(err)
	}
	return toGrant(row), nil
}

func (r *Repository) GetGrant(ctx context.Context, id string) (domain.Grant, error) {
	row, err := r.q.GetGrant(ctx, id)
	if err != nil {
		return domain.Grant{}, notFound(err, domain.ErrRefNotFound)
	}
	return toGrant(row), nil
}

func (r *Repository) UpdateGrant(ctx context.Context, id string, in domain.GrantInput) (domain.Grant, error) {
	row, err := r.q.UpdateGrant(ctx, educationsql.UpdateGrantParams{
		Title: ptrText(in.Title), Funder: text(in.Funder), FunderRef: text(in.FunderRef), Amount: numArg(in.Amount),
		Currency: text(in.Currency), StartOn: datePtr(in.StartOn), EndOn: datePtr(in.EndOn), Status: text(in.Status), ID: id,
	})
	if err != nil {
		return domain.Grant{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toGrant(row), nil
}

func (r *Repository) ListGrantsByInstitution(ctx context.Context, institutionID string) ([]domain.Grant, error) {
	rows, err := r.q.ListGrantsByInstitution(ctx, institutionID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Grant, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGrant(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteGrant(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteGrant(ctx, id)
}

// ---------------------------------------------------------------- publications

func (r *Repository) InsertPublication(ctx context.Context, in domain.PublicationInput) (domain.Publication, error) {
	row, err := r.q.InsertPublication(ctx, educationsql.InsertPublicationParams{
		InstitutionID: text(in.InstitutionID), Code: in.Code, Title: in.Title, Kind: iface(in.Kind),
		Doi: text(in.Doi), Venue: text(in.Venue), PublishedOn: datePtr(in.PublishedOn), OpenAccess: bool4(in.OpenAccess),
	})
	if err != nil {
		return domain.Publication{}, mapErr(err)
	}
	return toPublication(row), nil
}

func (r *Repository) GetPublication(ctx context.Context, id string) (domain.Publication, error) {
	row, err := r.q.GetPublication(ctx, id)
	if err != nil {
		return domain.Publication{}, notFound(err, domain.ErrRefNotFound)
	}
	return toPublication(row), nil
}

func (r *Repository) UpdatePublication(ctx context.Context, id string, in domain.PublicationInput) (domain.Publication, error) {
	row, err := r.q.UpdatePublication(ctx, educationsql.UpdatePublicationParams{
		Title: ptrText(in.Title), InstitutionID: text(in.InstitutionID), Kind: text(in.Kind), Doi: text(in.Doi),
		Venue: text(in.Venue), PublishedOn: datePtr(in.PublishedOn), OpenAccess: bool4(in.OpenAccess), ID: id,
	})
	if err != nil {
		return domain.Publication{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toPublication(row), nil
}

// ListPublications returns a keyset page of publications. A non-empty query routes to the dedicated
// trigram SearchPublications (review R-21) so the code/title match stays a GIN bitmap scan; the empty
// case is the plain keyset list. Both are `SELECT *`, so their rows share the model type.
func (r *Repository) ListPublications(ctx context.Context, query, after string, lim int) ([]domain.Publication, error) {
	var rows []educationsql.OikumeneaEducationPublication
	if q := strings.TrimSpace(query); q != "" {
		found, err := r.q.SearchPublications(ctx, educationsql.SearchPublicationsParams{Query: pgtype.Text{String: q, Valid: true}, After: after, Lim: int32(lim)})
		if err != nil {
			return nil, err
		}
		rows = found
	} else {
		var err error
		if rows, err = r.q.ListPublications(ctx, educationsql.ListPublicationsParams{After: after, Lim: int32(lim)}); err != nil {
			return nil, err
		}
	}
	out := make([]domain.Publication, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPublication(row))
	}
	return out, nil
}

func (r *Repository) SoftDeletePublication(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeletePublication(ctx, id)
}

// ---------------------------------------------------------------- governance bodies

func (r *Repository) InsertGovernanceBody(ctx context.Context, institutionID string, in domain.GovernanceBodyInput) (domain.GovernanceBody, error) {
	row, err := r.q.InsertGovernanceBody(ctx, educationsql.InsertGovernanceBodyParams{
		InstitutionID: institutionID, Code: in.Code, Name: in.Name, Kind: iface(in.Kind), Mandate: text(in.Mandate),
	})
	if err != nil {
		return domain.GovernanceBody{}, mapErr(err)
	}
	return toGovernanceBody(row), nil
}

func (r *Repository) GetGovernanceBody(ctx context.Context, id string) (domain.GovernanceBody, error) {
	row, err := r.q.GetGovernanceBody(ctx, id)
	if err != nil {
		return domain.GovernanceBody{}, notFound(err, domain.ErrRefNotFound)
	}
	return toGovernanceBody(row), nil
}

func (r *Repository) UpdateGovernanceBody(ctx context.Context, id string, in domain.GovernanceBodyInput) (domain.GovernanceBody, error) {
	row, err := r.q.UpdateGovernanceBody(ctx, educationsql.UpdateGovernanceBodyParams{
		Name: ptrText(in.Name), Kind: text(in.Kind), Mandate: text(in.Mandate), Status: text(in.Status), ID: id,
	})
	if err != nil {
		return domain.GovernanceBody{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toGovernanceBody(row), nil
}

func (r *Repository) ListGovernanceBodiesByInstitution(ctx context.Context, institutionID string) ([]domain.GovernanceBody, error) {
	rows, err := r.q.ListGovernanceBodiesByInstitution(ctx, institutionID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.GovernanceBody, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGovernanceBody(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteGovernanceBody(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteGovernanceBody(ctx, id)
}

// ---------------------------------------------------------------- policies

func (r *Repository) InsertPolicy(ctx context.Context, institutionID string, in domain.PolicyInput) (domain.Policy, error) {
	row, err := r.q.InsertPolicy(ctx, educationsql.InsertPolicyParams{
		InstitutionID: institutionID, GovernanceBodyID: text(in.GovernanceBodyID), SupersedesID: text(in.SupersedesID),
		Code: in.Code, Title: in.Title, Kind: iface(in.Kind), EffectiveOn: datePtr(in.EffectiveOn), ExpiryOn: datePtr(in.ExpiryOn), DocumentUrl: text(in.DocumentURL),
	})
	if err != nil {
		return domain.Policy{}, mapErr(err)
	}
	return toPolicy(row), nil
}

func (r *Repository) GetPolicy(ctx context.Context, id string) (domain.Policy, error) {
	row, err := r.q.GetPolicy(ctx, id)
	if err != nil {
		return domain.Policy{}, notFound(err, domain.ErrRefNotFound)
	}
	return toPolicy(row), nil
}

func (r *Repository) UpdatePolicy(ctx context.Context, id string, in domain.PolicyInput) (domain.Policy, error) {
	row, err := r.q.UpdatePolicy(ctx, educationsql.UpdatePolicyParams{
		Title: ptrText(in.Title), GovernanceBodyID: text(in.GovernanceBodyID), SupersedesID: text(in.SupersedesID), Kind: text(in.Kind),
		EffectiveOn: datePtr(in.EffectiveOn), ExpiryOn: datePtr(in.ExpiryOn), DocumentUrl: text(in.DocumentURL), Status: text(in.Status), ID: id,
	})
	if err != nil {
		return domain.Policy{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toPolicy(row), nil
}

func (r *Repository) ListPoliciesByInstitution(ctx context.Context, institutionID string) ([]domain.Policy, error) {
	rows, err := r.q.ListPoliciesByInstitution(ctx, institutionID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Policy, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPolicy(row))
	}
	return out, nil
}

func (r *Repository) SoftDeletePolicy(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeletePolicy(ctx, id)
}

// ---------------------------------------------------------------- qualifications

func (r *Repository) InsertQualification(ctx context.Context, institutionID string, in domain.QualificationInput) (domain.Qualification, error) {
	row, err := r.q.InsertQualification(ctx, educationsql.InsertQualificationParams{
		InstitutionID: institutionID, ProgramID: text(in.ProgramID), DegreeLevelID: text(in.DegreeLevelID), Code: in.Code, Name: in.Name,
		FrameworkCode: text(in.FrameworkCode), FrameworkLevel: text(in.FrameworkLevel), AwardingBody: text(in.AwardingBody),
	})
	if err != nil {
		return domain.Qualification{}, mapErr(err)
	}
	return toQualification(row), nil
}

func (r *Repository) GetQualification(ctx context.Context, id string) (domain.Qualification, error) {
	row, err := r.q.GetQualification(ctx, id)
	if err != nil {
		return domain.Qualification{}, notFound(err, domain.ErrRefNotFound)
	}
	return toQualification(row), nil
}

func (r *Repository) UpdateQualification(ctx context.Context, id string, in domain.QualificationInput) (domain.Qualification, error) {
	row, err := r.q.UpdateQualification(ctx, educationsql.UpdateQualificationParams{
		Name: ptrText(in.Name), ProgramID: text(in.ProgramID), DegreeLevelID: text(in.DegreeLevelID), FrameworkCode: text(in.FrameworkCode),
		FrameworkLevel: text(in.FrameworkLevel), AwardingBody: text(in.AwardingBody), Status: text(in.Status), ID: id,
	})
	if err != nil {
		return domain.Qualification{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toQualification(row), nil
}

func (r *Repository) ListQualificationsByInstitution(ctx context.Context, institutionID string) ([]domain.Qualification, error) {
	rows, err := r.q.ListQualificationsByInstitution(ctx, institutionID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Qualification, 0, len(rows))
	for _, row := range rows {
		out = append(out, toQualification(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteQualification(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteQualification(ctx, id)
}

// ---------------------------------------------------------------- scholarships

func (r *Repository) InsertScholarship(ctx context.Context, in domain.ScholarshipInput) (domain.Scholarship, error) {
	row, err := r.q.InsertScholarship(ctx, educationsql.InsertScholarshipParams{
		InstitutionID: text(in.InstitutionID), Code: in.Code, Name: in.Name, Kind: iface(in.Kind), Amount: numArg(in.Amount),
		Currency: text(in.Currency), Frequency: iface(in.Frequency), Renewable: bool4(in.Renewable), Conditions: text(in.Conditions),
	})
	if err != nil {
		return domain.Scholarship{}, mapErr(err)
	}
	return toScholarship(row), nil
}

func (r *Repository) GetScholarship(ctx context.Context, id string) (domain.Scholarship, error) {
	row, err := r.q.GetScholarship(ctx, id)
	if err != nil {
		return domain.Scholarship{}, notFound(err, domain.ErrRefNotFound)
	}
	return toScholarship(row), nil
}

func (r *Repository) UpdateScholarship(ctx context.Context, id string, in domain.ScholarshipInput) (domain.Scholarship, error) {
	row, err := r.q.UpdateScholarship(ctx, educationsql.UpdateScholarshipParams{
		Name: ptrText(in.Name), InstitutionID: text(in.InstitutionID), Kind: text(in.Kind), Amount: numArg(in.Amount), Currency: text(in.Currency),
		Frequency: text(in.Frequency), Renewable: bool4(in.Renewable), Conditions: text(in.Conditions), Status: text(in.Status), ID: id,
	})
	if err != nil {
		return domain.Scholarship{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toScholarship(row), nil
}

// ListScholarships returns a keyset page of scholarships. A non-empty query routes to the dedicated
// trigram SearchScholarships (review R-21) so the code/name match stays a GIN bitmap scan; the empty
// case is the plain keyset list. Both are `SELECT *`, so their rows share the model type.
func (r *Repository) ListScholarships(ctx context.Context, query, after string, lim int) ([]domain.Scholarship, error) {
	var rows []educationsql.OikumeneaEducationScholarship
	if q := strings.TrimSpace(query); q != "" {
		found, err := r.q.SearchScholarships(ctx, educationsql.SearchScholarshipsParams{Query: pgtype.Text{String: q, Valid: true}, After: after, Lim: int32(lim)})
		if err != nil {
			return nil, err
		}
		rows = found
	} else {
		var err error
		if rows, err = r.q.ListScholarships(ctx, educationsql.ListScholarshipsParams{After: after, Lim: int32(lim)}); err != nil {
			return nil, err
		}
	}
	out := make([]domain.Scholarship, 0, len(rows))
	for _, row := range rows {
		out = append(out, toScholarship(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteScholarship(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteScholarship(ctx, id)
}

// ---------------------------------------------------------------- accreditation events

func (r *Repository) InsertAccreditationEvent(ctx context.Context, in domain.AccreditationEventInput) (domain.AccreditationEvent, error) {
	row, err := r.q.InsertAccreditationEvent(ctx, educationsql.InsertAccreditationEventParams{
		EntityKind: in.EntityKind, InstitutionID: text(in.InstitutionID), ProgramID: text(in.ProgramID), Body: text(in.Body),
		BodyCountryID: text(in.BodyCountryID), Outcome: iface(in.Outcome), ReviewOn: datePtr(in.ReviewOn),
		EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo), Notes: text(in.Notes),
	})
	if err != nil {
		return domain.AccreditationEvent{}, mapErr(err)
	}
	return toAccreditationEvent(row), nil
}

func (r *Repository) GetAccreditationEvent(ctx context.Context, id string) (domain.AccreditationEvent, error) {
	row, err := r.q.GetAccreditationEvent(ctx, id)
	if err != nil {
		return domain.AccreditationEvent{}, notFound(err, domain.ErrRefNotFound)
	}
	return toAccreditationEvent(row), nil
}

func (r *Repository) UpdateAccreditationEvent(ctx context.Context, id string, in domain.AccreditationEventInput) (domain.AccreditationEvent, error) {
	row, err := r.q.UpdateAccreditationEvent(ctx, educationsql.UpdateAccreditationEventParams{
		Body: text(in.Body), BodyCountryID: text(in.BodyCountryID), Outcome: text(in.Outcome), ReviewOn: datePtr(in.ReviewOn),
		EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo), Notes: text(in.Notes), ID: id,
	})
	if err != nil {
		return domain.AccreditationEvent{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toAccreditationEvent(row), nil
}

func (r *Repository) ListAccreditationEvents(ctx context.Context, institutionID, programID string) ([]domain.AccreditationEvent, error) {
	rows, err := r.q.ListAccreditationEvents(ctx, educationsql.ListAccreditationEventsParams{InstitutionID: institutionID, ProgramID: programID})
	if err != nil {
		return nil, err
	}
	out := make([]domain.AccreditationEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAccreditationEvent(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteAccreditationEvent(ctx context.Context, id string) (int64, error) {
	return r.q.SoftDeleteAccreditationEvent(ctx, id)
}

// ---------------------------------------------------------------- person: publication authorships

func (r *Repository) InsertPublicationAuthorship(ctx context.Context, personID string, in domain.PublicationAuthorshipInput) (domain.PublicationAuthorship, error) {
	row, err := r.q.InsertPublicationAuthorship(ctx, educationsql.InsertPublicationAuthorshipParams{
		PersonID: personID, PublicationID: in.PublicationID, AuthorOrder: int4(in.AuthorOrder), Corresponding: bool4(in.Corresponding),
		EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo),
	})
	if err != nil {
		return domain.PublicationAuthorship{}, mapErr(err)
	}
	return toAuthorship(row), nil
}

func (r *Repository) UpdatePublicationAuthorship(ctx context.Context, personID, id string, in domain.PublicationAuthorshipInput) (domain.PublicationAuthorship, error) {
	row, err := r.q.UpdatePublicationAuthorship(ctx, educationsql.UpdatePublicationAuthorshipParams{
		PublicationID: ptrTextIf(in.PublicationID), AuthorOrder: int4(in.AuthorOrder), Corresponding: bool4(in.Corresponding),
		EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo), ID: id, PersonID: personID,
	})
	if err != nil {
		return domain.PublicationAuthorship{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toAuthorship(row), nil
}

func (r *Repository) SoftDeletePublicationAuthorship(ctx context.Context, personID, id string) (int64, error) {
	return r.q.SoftDeletePublicationAuthorship(ctx, educationsql.SoftDeletePublicationAuthorshipParams{ID: id, PersonID: personID})
}

func (r *Repository) ListPublicationAuthorshipsByPerson(ctx context.Context, personID string) ([]domain.PublicationAuthorship, error) {
	rows, err := r.q.ListPublicationAuthorshipsByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PublicationAuthorship, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAuthorship(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- person: research memberships

func (r *Repository) InsertResearchMembership(ctx context.Context, personID string, in domain.ResearchMembershipInput) (domain.ResearchMembership, error) {
	row, err := r.q.InsertResearchMembership(ctx, educationsql.InsertResearchMembershipParams{
		PersonID: personID, GroupID: in.GroupID, Role: text(in.Role), Status: iface(in.Status), EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo),
	})
	if err != nil {
		return domain.ResearchMembership{}, mapErr(err)
	}
	return toResearchMembership(row), nil
}

func (r *Repository) UpdateResearchMembership(ctx context.Context, personID, id string, in domain.ResearchMembershipInput) (domain.ResearchMembership, error) {
	row, err := r.q.UpdateResearchMembership(ctx, educationsql.UpdateResearchMembershipParams{
		GroupID: ptrTextIf(in.GroupID), Role: text(in.Role), Status: text(in.Status), EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo), ID: id, PersonID: personID,
	})
	if err != nil {
		return domain.ResearchMembership{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toResearchMembership(row), nil
}

func (r *Repository) SoftDeleteResearchMembership(ctx context.Context, personID, id string) (int64, error) {
	return r.q.SoftDeleteResearchMembership(ctx, educationsql.SoftDeleteResearchMembershipParams{ID: id, PersonID: personID})
}

func (r *Repository) ListResearchMembershipsByPerson(ctx context.Context, personID string) ([]domain.ResearchMembership, error) {
	rows, err := r.q.ListResearchMembershipsByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ResearchMembership, 0, len(rows))
	for _, row := range rows {
		out = append(out, toResearchMembership(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- person: grant holdings

func (r *Repository) InsertGrantHolding(ctx context.Context, personID string, in domain.GrantHoldingInput) (domain.GrantHolding, error) {
	row, err := r.q.InsertGrantHolding(ctx, educationsql.InsertGrantHoldingParams{
		PersonID: personID, GrantID: in.GrantID, Role: iface(in.Role), Status: iface(in.Status), EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo),
	})
	if err != nil {
		return domain.GrantHolding{}, mapErr(err)
	}
	return toGrantHolding(row), nil
}

func (r *Repository) UpdateGrantHolding(ctx context.Context, personID, id string, in domain.GrantHoldingInput) (domain.GrantHolding, error) {
	row, err := r.q.UpdateGrantHolding(ctx, educationsql.UpdateGrantHoldingParams{
		GrantID: ptrTextIf(in.GrantID), Role: text(in.Role), Status: text(in.Status), EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo), ID: id, PersonID: personID,
	})
	if err != nil {
		return domain.GrantHolding{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toGrantHolding(row), nil
}

func (r *Repository) SoftDeleteGrantHolding(ctx context.Context, personID, id string) (int64, error) {
	return r.q.SoftDeleteGrantHolding(ctx, educationsql.SoftDeleteGrantHoldingParams{ID: id, PersonID: personID})
}

func (r *Repository) ListGrantHoldingsByPerson(ctx context.Context, personID string) ([]domain.GrantHolding, error) {
	rows, err := r.q.ListGrantHoldingsByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.GrantHolding, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGrantHolding(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- person: governance memberships

func (r *Repository) InsertGovernanceMembership(ctx context.Context, personID string, in domain.GovernanceMembershipInput) (domain.GovernanceMembership, error) {
	row, err := r.q.InsertGovernanceMembership(ctx, educationsql.InsertGovernanceMembershipParams{
		PersonID: personID, BodyID: in.BodyID, RoleInBody: text(in.RoleInBody), Status: iface(in.Status), EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo),
	})
	if err != nil {
		return domain.GovernanceMembership{}, mapErr(err)
	}
	return toGovernanceMembership(row), nil
}

func (r *Repository) UpdateGovernanceMembership(ctx context.Context, personID, id string, in domain.GovernanceMembershipInput) (domain.GovernanceMembership, error) {
	row, err := r.q.UpdateGovernanceMembership(ctx, educationsql.UpdateGovernanceMembershipParams{
		BodyID: ptrTextIf(in.BodyID), RoleInBody: text(in.RoleInBody), Status: text(in.Status), EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo), ID: id, PersonID: personID,
	})
	if err != nil {
		return domain.GovernanceMembership{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toGovernanceMembership(row), nil
}

func (r *Repository) SoftDeleteGovernanceMembership(ctx context.Context, personID, id string) (int64, error) {
	return r.q.SoftDeleteGovernanceMembership(ctx, educationsql.SoftDeleteGovernanceMembershipParams{ID: id, PersonID: personID})
}

func (r *Repository) ListGovernanceMembershipsByPerson(ctx context.Context, personID string) ([]domain.GovernanceMembership, error) {
	rows, err := r.q.ListGovernanceMembershipsByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.GovernanceMembership, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGovernanceMembership(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- person: qualification awards

func (r *Repository) InsertQualificationAward(ctx context.Context, personID string, in domain.QualificationAwardInput) (domain.QualificationAward, error) {
	row, err := r.q.InsertQualificationAward(ctx, educationsql.InsertQualificationAwardParams{
		PersonID: personID, QualificationID: in.QualificationID, EnrollmentID: text(in.EnrollmentID), AwardedOn: datePtr(in.AwardedOn),
		WithDistinction: bool4(in.WithDistinction), Gpa: numArg(in.Gpa), Status: iface(in.Status),
	})
	if err != nil {
		return domain.QualificationAward{}, mapErr(err)
	}
	return toQualificationAward(row), nil
}

func (r *Repository) UpdateQualificationAward(ctx context.Context, personID, id string, in domain.QualificationAwardInput) (domain.QualificationAward, error) {
	row, err := r.q.UpdateQualificationAward(ctx, educationsql.UpdateQualificationAwardParams{
		QualificationID: ptrTextIf(in.QualificationID), EnrollmentID: text(in.EnrollmentID), AwardedOn: datePtr(in.AwardedOn),
		WithDistinction: bool4(in.WithDistinction), Gpa: numArg(in.Gpa), Status: text(in.Status), ID: id, PersonID: personID,
	})
	if err != nil {
		return domain.QualificationAward{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toQualificationAward(row), nil
}

func (r *Repository) SoftDeleteQualificationAward(ctx context.Context, personID, id string) (int64, error) {
	return r.q.SoftDeleteQualificationAward(ctx, educationsql.SoftDeleteQualificationAwardParams{ID: id, PersonID: personID})
}

func (r *Repository) ListQualificationAwardsByPerson(ctx context.Context, personID string) ([]domain.QualificationAward, error) {
	rows, err := r.q.ListQualificationAwardsByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.QualificationAward, 0, len(rows))
	for _, row := range rows {
		out = append(out, toQualificationAward(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- person: scholarship awards

func (r *Repository) InsertScholarshipAward(ctx context.Context, personID string, in domain.ScholarshipAwardInput) (domain.ScholarshipAward, error) {
	row, err := r.q.InsertScholarshipAward(ctx, educationsql.InsertScholarshipAwardParams{
		PersonID: personID, ScholarshipID: in.ScholarshipID, Status: iface(in.Status), EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo),
	})
	if err != nil {
		return domain.ScholarshipAward{}, mapErr(err)
	}
	return toScholarshipAward(row), nil
}

func (r *Repository) UpdateScholarshipAward(ctx context.Context, personID, id string, in domain.ScholarshipAwardInput) (domain.ScholarshipAward, error) {
	row, err := r.q.UpdateScholarshipAward(ctx, educationsql.UpdateScholarshipAwardParams{
		ScholarshipID: ptrTextIf(in.ScholarshipID), Status: text(in.Status), EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo), ID: id, PersonID: personID,
	})
	if err != nil {
		return domain.ScholarshipAward{}, notFound(mapErr(err), domain.ErrRefNotFound)
	}
	return toScholarshipAward(row), nil
}

func (r *Repository) SoftDeleteScholarshipAward(ctx context.Context, personID, id string) (int64, error) {
	return r.q.SoftDeleteScholarshipAward(ctx, educationsql.SoftDeleteScholarshipAwardParams{ID: id, PersonID: personID})
}

func (r *Repository) ListScholarshipAwardsByPerson(ctx context.Context, personID string) ([]domain.ScholarshipAward, error) {
	rows, err := r.q.ListScholarshipAwardsByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ScholarshipAward, 0, len(rows))
	for _, row := range rows {
		out = append(out, toScholarshipAward(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- converters

func toProgram(r educationsql.OikumeneaEducationProgram) domain.Program {
	return domain.Program{
		ID: r.ID, InstitutionID: r.InstitutionID, OwningUnitID: strp(r.OwningUnitID), DegreeLevelID: strp(r.DegreeLevelID),
		Code: r.Code, Name: r.Name, Mode: r.Mode, DurationYears: numStr(r.DurationYears), CreditHoursTotal: int4ptr(r.CreditHoursTotal),
		State: r.State, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toCourse(r educationsql.OikumeneaEducationCourse) domain.Course {
	return domain.Course{
		ID: r.ID, InstitutionID: r.InstitutionID, OwningUnitID: strp(r.OwningUnitID), Code: r.Code, Title: r.Title,
		CreditHours: int4ptr(r.CreditHours), Level: int4ptr(r.Level), Description: strp(r.Description), DeliveryMode: r.DeliveryMode,
		Status: r.Status, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toCurriculumVersion(r educationsql.OikumeneaEducationCurriculumVersion) domain.CurriculumVersion {
	return domain.CurriculumVersion{
		ID: r.ID, ProgramID: r.ProgramID, VersionCode: r.VersionCode, EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo),
		Status: r.Status, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toCurriculumItem(r educationsql.OikumeneaEducationCurriculumItem) domain.CurriculumItem {
	return domain.CurriculumItem{
		ID: r.ID, VersionID: r.VersionID, CourseID: r.CourseID, IsRequired: r.IsRequired,
		YearOfStudy: int4ptr(r.YearOfStudy), CreditAllocation: int4ptr(r.CreditAllocation), SemesterSlot: int4ptr(r.SemesterSlot),
		CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toCoursePrerequisite(r educationsql.OikumeneaEducationCoursePrerequisite) domain.CoursePrerequisite {
	return domain.CoursePrerequisite{
		ID: r.ID, CourseID: r.CourseID, RequiredCourseID: r.RequiredCourseID, Kind: r.Kind, MinGrade: strp(r.MinGrade),
		CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toResearchCentre(r educationsql.OikumeneaEducationResearchCentre) domain.ResearchCentre {
	return domain.ResearchCentre{
		ID: r.ID, InstitutionID: r.InstitutionID, Code: r.Code, Name: r.Name, Kind: r.Kind, FundingSource: strp(r.FundingSource),
		FoundedOn: dateStr(r.FoundedOn), DissolvedOn: dateStr(r.DissolvedOn), Status: r.Status, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toResearchGroup(r educationsql.OikumeneaEducationResearchGroup) domain.ResearchGroup {
	return domain.ResearchGroup{
		ID: r.ID, InstitutionID: r.InstitutionID, CentreID: strp(r.CentreID), UnitID: strp(r.UnitID), Code: r.Code, Name: r.Name,
		FocusArea: strp(r.FocusArea), Status: r.Status, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toGrant(r educationsql.OikumeneaEducationGrant) domain.Grant {
	return domain.Grant{
		ID: r.ID, InstitutionID: r.InstitutionID, Code: r.Code, Title: r.Title, Funder: strp(r.Funder), FunderRef: strp(r.FunderRef),
		Amount: numStr(r.Amount), Currency: strp(r.Currency), StartOn: dateStr(r.StartOn), EndOn: dateStr(r.EndOn), Status: r.Status,
		CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toPublication(r educationsql.OikumeneaEducationPublication) domain.Publication {
	return domain.Publication{
		ID: r.ID, InstitutionID: strp(r.InstitutionID), Code: r.Code, Title: r.Title, Kind: r.Kind, Doi: strp(r.Doi), Venue: strp(r.Venue),
		PublishedOn: dateStr(r.PublishedOn), OpenAccess: r.OpenAccess, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toGovernanceBody(r educationsql.OikumeneaEducationGovernanceBody) domain.GovernanceBody {
	return domain.GovernanceBody{
		ID: r.ID, InstitutionID: r.InstitutionID, Code: r.Code, Name: r.Name, Kind: r.Kind, Mandate: strp(r.Mandate),
		Status: r.Status, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toPolicy(r educationsql.OikumeneaEducationPolicy) domain.Policy {
	return domain.Policy{
		ID: r.ID, InstitutionID: r.InstitutionID, GovernanceBodyID: strp(r.GovernanceBodyID), SupersedesID: strp(r.SupersedesID),
		Code: r.Code, Title: r.Title, Kind: r.Kind, EffectiveOn: dateStr(r.EffectiveOn), ExpiryOn: dateStr(r.ExpiryOn),
		DocumentURL: strp(r.DocumentUrl), Status: r.Status, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toQualification(r educationsql.OikumeneaEducationQualification) domain.Qualification {
	return domain.Qualification{
		ID: r.ID, InstitutionID: r.InstitutionID, ProgramID: strp(r.ProgramID), DegreeLevelID: strp(r.DegreeLevelID), Code: r.Code, Name: r.Name,
		FrameworkCode: strp(r.FrameworkCode), FrameworkLevel: strp(r.FrameworkLevel), AwardingBody: strp(r.AwardingBody), Status: r.Status,
		CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toScholarship(r educationsql.OikumeneaEducationScholarship) domain.Scholarship {
	return domain.Scholarship{
		ID: r.ID, InstitutionID: strp(r.InstitutionID), Code: r.Code, Name: r.Name, Kind: r.Kind, Amount: numStr(r.Amount), Currency: strp(r.Currency),
		Frequency: r.Frequency, Renewable: r.Renewable, Conditions: strp(r.Conditions), Status: r.Status, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toAccreditationEvent(r educationsql.OikumeneaEducationAccreditationEvent) domain.AccreditationEvent {
	return domain.AccreditationEvent{
		ID: r.ID, EntityKind: r.EntityKind, InstitutionID: strp(r.InstitutionID), ProgramID: strp(r.ProgramID), Body: strp(r.Body),
		BodyCountryID: strp(r.BodyCountryID), Outcome: r.Outcome, ReviewOn: dateStr(r.ReviewOn), EffectiveFrom: dateStr(r.EffectiveFrom),
		EffectiveTo: dateStr(r.EffectiveTo), Notes: strp(r.Notes), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toAuthorship(r educationsql.OikumeneaPersonPublicationAuthorship) domain.PublicationAuthorship {
	return domain.PublicationAuthorship{
		ID: r.ID, PersonID: r.PersonID, PublicationID: r.PublicationID, AuthorOrder: int4ptr(r.AuthorOrder), Corresponding: r.Corresponding,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toResearchMembership(r educationsql.OikumeneaPersonResearchMembership) domain.ResearchMembership {
	return domain.ResearchMembership{
		ID: r.ID, PersonID: r.PersonID, GroupID: r.GroupID, Role: strp(r.Role), Status: r.Status,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toGrantHolding(r educationsql.OikumeneaPersonGrantHolding) domain.GrantHolding {
	return domain.GrantHolding{
		ID: r.ID, PersonID: r.PersonID, GrantID: r.GrantID, Role: r.Role, Status: r.Status,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toGovernanceMembership(r educationsql.OikumeneaPersonGovernanceMembership) domain.GovernanceMembership {
	return domain.GovernanceMembership{
		ID: r.ID, PersonID: r.PersonID, BodyID: r.BodyID, RoleInBody: strp(r.RoleInBody), Status: r.Status,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toQualificationAward(r educationsql.OikumeneaPersonEducationQualification) domain.QualificationAward {
	return domain.QualificationAward{
		ID: r.ID, PersonID: r.PersonID, QualificationID: r.QualificationID, EnrollmentID: strp(r.EnrollmentID), AwardedOn: dateStr(r.AwardedOn),
		WithDistinction: r.WithDistinction, Gpa: numStr(r.Gpa), Status: r.Status, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toScholarshipAward(r educationsql.OikumeneaPersonScholarshipAward) domain.ScholarshipAward {
	return domain.ScholarshipAward{
		ID: r.ID, PersonID: r.PersonID, ScholarshipID: r.ScholarshipID, Status: r.Status,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

// ---------------------------------------------------------------- reference-layer helpers

// numStr renders a nullable numeric as its text form ("" when NULL).
func numStr(n pgtype.Numeric) string {
	if !n.Valid {
		return ""
	}
	v, err := n.Value()
	if err != nil || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// numArg maps an optional text amount to a nullable numeric param (nil/"" => NULL).
func numArg(p *string) pgtype.Numeric {
	var n pgtype.Numeric
	if p == nil || *p == "" {
		return n
	}
	if err := n.Scan(*p); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// bool4 maps an optional bool to a nullable bool param (nil => leave SQL default/unchanged).
func bool4(p *bool) pgtype.Bool {
	if p == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *p, Valid: true}
}

// ptrText wraps a required string as a present pgtype.Text (always set — used for update-name etc.).
func ptrText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// ptrTextIf wraps a string as pgtype.Text, treating "" as absent (NULL => COALESCE keeps the column).
func ptrTextIf(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
