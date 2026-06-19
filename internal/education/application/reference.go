package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/education/domain"
)

// Reference-layer orchestration (M20 extension): audited CRUD for the curriculum/research/governance/
// credentials reference entities + the person-binding links. Mirrors service.go (inTx + record). Reads
// run on the pool; writes record one `system` Action in the same transaction (D-Audit).

// ---- generic create/update/delete helpers (audited) ----

func (s *Service) refDelete(ctx context.Context, action, id string, del func(domain.Repository) (int64, error)) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := del(s.newRepo(tx))
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrRefNotFound
		}
		return s.record(ctx, tx, action, id, map[string]string{"id": id})
	})
}

// ============================ programs ============================

func (s *Service) CreateProgram(ctx context.Context, institutionID string, in domain.ProgramInput) (domain.Program, error) {
	if err := in.Validate(); err != nil {
		return domain.Program{}, err
	}
	var out domain.Program
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertProgram(ctx, institutionID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.program.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetProgram(ctx context.Context, id string) (domain.Program, error) {
	return s.newRepo(s.pool).GetProgram(ctx, id)
}

func (s *Service) ListPrograms(ctx context.Context, institutionID string) ([]domain.Program, error) {
	return s.newRepo(s.pool).ListProgramsByInstitution(ctx, institutionID)
}

func (s *Service) UpdateProgram(ctx context.Context, id string, in domain.ProgramInput) (domain.Program, error) {
	var out domain.Program
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateProgram(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.program.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteProgram(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.program.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteProgram(ctx, id) })
}

// ============================ courses ============================

func (s *Service) CreateCourse(ctx context.Context, institutionID string, in domain.CourseInput) (domain.Course, error) {
	if err := in.Validate(); err != nil {
		return domain.Course{}, err
	}
	var out domain.Course
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertCourse(ctx, institutionID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.course.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetCourse(ctx context.Context, id string) (domain.Course, error) {
	return s.newRepo(s.pool).GetCourse(ctx, id)
}

func (s *Service) ListCourses(ctx context.Context, institutionID string) ([]domain.Course, error) {
	return s.newRepo(s.pool).ListCoursesByInstitution(ctx, institutionID)
}

func (s *Service) UpdateCourse(ctx context.Context, id string, in domain.CourseInput) (domain.Course, error) {
	var out domain.Course
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateCourse(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.course.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteCourse(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.course.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteCourse(ctx, id) })
}

// ============================ curriculum versions ============================

func (s *Service) CreateCurriculumVersion(ctx context.Context, programID string, in domain.CurriculumVersionInput) (domain.CurriculumVersion, error) {
	if err := in.Validate(); err != nil {
		return domain.CurriculumVersion{}, err
	}
	var out domain.CurriculumVersion
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertCurriculumVersion(ctx, programID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.curriculum-version.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetCurriculumVersion(ctx context.Context, id string) (domain.CurriculumVersion, error) {
	return s.newRepo(s.pool).GetCurriculumVersion(ctx, id)
}

func (s *Service) ListCurriculumVersions(ctx context.Context, programID string) ([]domain.CurriculumVersion, error) {
	return s.newRepo(s.pool).ListCurriculumVersionsByProgram(ctx, programID)
}

func (s *Service) UpdateCurriculumVersion(ctx context.Context, id string, in domain.CurriculumVersionInput) (domain.CurriculumVersion, error) {
	var out domain.CurriculumVersion
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateCurriculumVersion(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.curriculum-version.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteCurriculumVersion(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.curriculum-version.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteCurriculumVersion(ctx, id) })
}

// ============================ curriculum items ============================

func (s *Service) AddCurriculumItem(ctx context.Context, versionID string, in domain.CurriculumItemInput) (domain.CurriculumItem, error) {
	if err := in.Validate(); err != nil {
		return domain.CurriculumItem{}, err
	}
	var out domain.CurriculumItem
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertCurriculumItem(ctx, versionID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.curriculum-item.create", v.ID, v)
	})
	return out, err
}

func (s *Service) ListCurriculumItems(ctx context.Context, versionID string) ([]domain.CurriculumItem, error) {
	return s.newRepo(s.pool).ListCurriculumItemsByVersion(ctx, versionID)
}

func (s *Service) UpdateCurriculumItem(ctx context.Context, id string, in domain.CurriculumItemInput) (domain.CurriculumItem, error) {
	var out domain.CurriculumItem
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateCurriculumItem(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.curriculum-item.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteCurriculumItem(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.curriculum-item.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteCurriculumItem(ctx, id) })
}

// ============================ course prerequisites ============================

// AddCoursePrerequisite adds a prerequisite edge after a Go-side cycle guard: a new edge C→R is rejected
// if R already (transitively) requires C (which would close a cycle).
func (s *Service) AddCoursePrerequisite(ctx context.Context, courseID string, in domain.CoursePrerequisiteInput) (domain.CoursePrerequisite, error) {
	if err := in.Validate(); err != nil {
		return domain.CoursePrerequisite{}, err
	}
	if courseID == in.RequiredCourseID {
		return domain.CoursePrerequisite{}, domain.ErrPrereqCycle
	}
	var out domain.CoursePrerequisite
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		edges, err := repo.ListPrerequisiteEdges(ctx)
		if err != nil {
			return err
		}
		if prereqReaches(edges, in.RequiredCourseID, courseID) {
			return domain.ErrPrereqCycle
		}
		v, err := repo.InsertCoursePrerequisite(ctx, courseID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.course-prerequisite.create", v.ID, v)
	})
	return out, err
}

func (s *Service) ListCoursePrerequisites(ctx context.Context, courseID string) ([]domain.CoursePrerequisite, error) {
	return s.newRepo(s.pool).ListCoursePrerequisitesByCourse(ctx, courseID)
}

func (s *Service) DeleteCoursePrerequisite(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.course-prerequisite.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteCoursePrerequisite(ctx, id) })
}

// prereqReaches reports whether `from` can reach `target` by following course→required edges.
func prereqReaches(edges []domain.PrereqEdge, from, target string) bool {
	adj := map[string][]string{}
	for _, e := range edges {
		adj[e.CourseID] = append(adj[e.CourseID], e.RequiredCourseID)
	}
	seen := map[string]bool{}
	var dfs func(n string) bool
	dfs = func(n string) bool {
		if n == target {
			return true
		}
		if seen[n] {
			return false
		}
		seen[n] = true
		for _, nx := range adj[n] {
			if dfs(nx) {
				return true
			}
		}
		return false
	}
	return dfs(from)
}

// ============================ research centres ============================

func (s *Service) CreateResearchCentre(ctx context.Context, institutionID string, in domain.ResearchCentreInput) (domain.ResearchCentre, error) {
	if err := in.Validate(); err != nil {
		return domain.ResearchCentre{}, err
	}
	var out domain.ResearchCentre
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertResearchCentre(ctx, institutionID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.research-centre.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetResearchCentre(ctx context.Context, id string) (domain.ResearchCentre, error) {
	return s.newRepo(s.pool).GetResearchCentre(ctx, id)
}

func (s *Service) ListResearchCentres(ctx context.Context, institutionID string) ([]domain.ResearchCentre, error) {
	return s.newRepo(s.pool).ListResearchCentresByInstitution(ctx, institutionID)
}

func (s *Service) UpdateResearchCentre(ctx context.Context, id string, in domain.ResearchCentreInput) (domain.ResearchCentre, error) {
	var out domain.ResearchCentre
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateResearchCentre(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.research-centre.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteResearchCentre(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.research-centre.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteResearchCentre(ctx, id) })
}

// ============================ research groups ============================

func (s *Service) CreateResearchGroup(ctx context.Context, institutionID string, in domain.ResearchGroupInput) (domain.ResearchGroup, error) {
	if err := in.Validate(); err != nil {
		return domain.ResearchGroup{}, err
	}
	var out domain.ResearchGroup
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertResearchGroup(ctx, institutionID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.research-group.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetResearchGroup(ctx context.Context, id string) (domain.ResearchGroup, error) {
	return s.newRepo(s.pool).GetResearchGroup(ctx, id)
}

func (s *Service) ListResearchGroups(ctx context.Context, institutionID string) ([]domain.ResearchGroup, error) {
	return s.newRepo(s.pool).ListResearchGroupsByInstitution(ctx, institutionID)
}

func (s *Service) UpdateResearchGroup(ctx context.Context, id string, in domain.ResearchGroupInput) (domain.ResearchGroup, error) {
	var out domain.ResearchGroup
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateResearchGroup(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.research-group.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteResearchGroup(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.research-group.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteResearchGroup(ctx, id) })
}

// ============================ grants ============================

func (s *Service) CreateGrant(ctx context.Context, institutionID string, in domain.GrantInput) (domain.Grant, error) {
	if err := in.Validate(); err != nil {
		return domain.Grant{}, err
	}
	var out domain.Grant
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertGrant(ctx, institutionID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.grant.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetGrant(ctx context.Context, id string) (domain.Grant, error) {
	return s.newRepo(s.pool).GetGrant(ctx, id)
}

func (s *Service) ListGrants(ctx context.Context, institutionID string) ([]domain.Grant, error) {
	return s.newRepo(s.pool).ListGrantsByInstitution(ctx, institutionID)
}

func (s *Service) UpdateGrant(ctx context.Context, id string, in domain.GrantInput) (domain.Grant, error) {
	var out domain.Grant
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateGrant(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.grant.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteGrant(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.grant.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteGrant(ctx, id) })
}

// ============================ publications ============================

func (s *Service) CreatePublication(ctx context.Context, in domain.PublicationInput) (domain.Publication, error) {
	if err := in.Validate(); err != nil {
		return domain.Publication{}, err
	}
	var out domain.Publication
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertPublication(ctx, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.publication.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetPublication(ctx context.Context, id string) (domain.Publication, error) {
	return s.newRepo(s.pool).GetPublication(ctx, id)
}

func (s *Service) ListPublications(ctx context.Context, query string) ([]domain.Publication, error) {
	return s.newRepo(s.pool).ListPublications(ctx, query)
}

func (s *Service) UpdatePublication(ctx context.Context, id string, in domain.PublicationInput) (domain.Publication, error) {
	var out domain.Publication
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdatePublication(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.publication.update", id, v)
	})
	return out, err
}

func (s *Service) DeletePublication(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.publication.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeletePublication(ctx, id) })
}

// ============================ governance bodies ============================

func (s *Service) CreateGovernanceBody(ctx context.Context, institutionID string, in domain.GovernanceBodyInput) (domain.GovernanceBody, error) {
	if err := in.Validate(); err != nil {
		return domain.GovernanceBody{}, err
	}
	var out domain.GovernanceBody
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertGovernanceBody(ctx, institutionID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.governance-body.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetGovernanceBody(ctx context.Context, id string) (domain.GovernanceBody, error) {
	return s.newRepo(s.pool).GetGovernanceBody(ctx, id)
}

func (s *Service) ListGovernanceBodies(ctx context.Context, institutionID string) ([]domain.GovernanceBody, error) {
	return s.newRepo(s.pool).ListGovernanceBodiesByInstitution(ctx, institutionID)
}

func (s *Service) UpdateGovernanceBody(ctx context.Context, id string, in domain.GovernanceBodyInput) (domain.GovernanceBody, error) {
	var out domain.GovernanceBody
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateGovernanceBody(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.governance-body.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteGovernanceBody(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.governance-body.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteGovernanceBody(ctx, id) })
}

// ============================ policies ============================

func (s *Service) CreatePolicy(ctx context.Context, institutionID string, in domain.PolicyInput) (domain.Policy, error) {
	if err := in.Validate(); err != nil {
		return domain.Policy{}, err
	}
	var out domain.Policy
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertPolicy(ctx, institutionID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.policy.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetPolicy(ctx context.Context, id string) (domain.Policy, error) {
	return s.newRepo(s.pool).GetPolicy(ctx, id)
}

func (s *Service) ListPolicies(ctx context.Context, institutionID string) ([]domain.Policy, error) {
	return s.newRepo(s.pool).ListPoliciesByInstitution(ctx, institutionID)
}

func (s *Service) UpdatePolicy(ctx context.Context, id string, in domain.PolicyInput) (domain.Policy, error) {
	var out domain.Policy
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdatePolicy(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.policy.update", id, v)
	})
	return out, err
}

func (s *Service) DeletePolicy(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.policy.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeletePolicy(ctx, id) })
}

// ============================ qualifications ============================

func (s *Service) CreateQualification(ctx context.Context, institutionID string, in domain.QualificationInput) (domain.Qualification, error) {
	if err := in.Validate(); err != nil {
		return domain.Qualification{}, err
	}
	var out domain.Qualification
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertQualification(ctx, institutionID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.qualification.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetQualification(ctx context.Context, id string) (domain.Qualification, error) {
	return s.newRepo(s.pool).GetQualification(ctx, id)
}

func (s *Service) ListQualifications(ctx context.Context, institutionID string) ([]domain.Qualification, error) {
	return s.newRepo(s.pool).ListQualificationsByInstitution(ctx, institutionID)
}

func (s *Service) UpdateQualification(ctx context.Context, id string, in domain.QualificationInput) (domain.Qualification, error) {
	var out domain.Qualification
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateQualification(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.qualification.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteQualification(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.qualification.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteQualification(ctx, id) })
}

// ============================ scholarships ============================

func (s *Service) CreateScholarship(ctx context.Context, in domain.ScholarshipInput) (domain.Scholarship, error) {
	if err := in.Validate(); err != nil {
		return domain.Scholarship{}, err
	}
	var out domain.Scholarship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertScholarship(ctx, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.scholarship.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetScholarship(ctx context.Context, id string) (domain.Scholarship, error) {
	return s.newRepo(s.pool).GetScholarship(ctx, id)
}

func (s *Service) ListScholarships(ctx context.Context, query string) ([]domain.Scholarship, error) {
	return s.newRepo(s.pool).ListScholarships(ctx, query)
}

func (s *Service) UpdateScholarship(ctx context.Context, id string, in domain.ScholarshipInput) (domain.Scholarship, error) {
	var out domain.Scholarship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateScholarship(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.scholarship.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteScholarship(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.scholarship.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteScholarship(ctx, id) })
}

// ============================ accreditation events ============================

func (s *Service) CreateAccreditationEvent(ctx context.Context, in domain.AccreditationEventInput) (domain.AccreditationEvent, error) {
	if err := in.Validate(); err != nil {
		return domain.AccreditationEvent{}, err
	}
	var out domain.AccreditationEvent
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertAccreditationEvent(ctx, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.accreditation-event.create", v.ID, v)
	})
	return out, err
}

func (s *Service) GetAccreditationEvent(ctx context.Context, id string) (domain.AccreditationEvent, error) {
	return s.newRepo(s.pool).GetAccreditationEvent(ctx, id)
}

func (s *Service) ListAccreditationEvents(ctx context.Context, institutionID, programID string) ([]domain.AccreditationEvent, error) {
	return s.newRepo(s.pool).ListAccreditationEvents(ctx, institutionID, programID)
}

func (s *Service) UpdateAccreditationEvent(ctx context.Context, id string, in domain.AccreditationEventInput) (domain.AccreditationEvent, error) {
	var out domain.AccreditationEvent
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateAccreditationEvent(ctx, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.accreditation-event.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteAccreditationEvent(ctx context.Context, id string) error {
	return s.refDelete(ctx, "education.accreditation-event.delete", id, func(r domain.Repository) (int64, error) { return r.SoftDeleteAccreditationEvent(ctx, id) })
}

// ============================ person: publication authorships ============================

func (s *Service) ListPublicationAuthorships(ctx context.Context, personID string) ([]domain.PublicationAuthorship, error) {
	return s.newRepo(s.pool).ListPublicationAuthorshipsByPerson(ctx, personID)
}

func (s *Service) CreatePublicationAuthorship(ctx context.Context, personID string, in domain.PublicationAuthorshipInput) (domain.PublicationAuthorship, error) {
	if err := in.Validate(); err != nil {
		return domain.PublicationAuthorship{}, err
	}
	var out domain.PublicationAuthorship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertPublicationAuthorship(ctx, personID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.publication-authorship.create", v.ID, v)
	})
	return out, err
}

func (s *Service) UpdatePublicationAuthorship(ctx context.Context, personID, id string, in domain.PublicationAuthorshipInput) (domain.PublicationAuthorship, error) {
	var out domain.PublicationAuthorship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdatePublicationAuthorship(ctx, personID, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.publication-authorship.update", id, v)
	})
	return out, err
}

func (s *Service) DeletePublicationAuthorship(ctx context.Context, personID, id string) error {
	return s.refDelete(ctx, "education.publication-authorship.delete", id, func(r domain.Repository) (int64, error) {
		return r.SoftDeletePublicationAuthorship(ctx, personID, id)
	})
}

// ============================ person: research memberships ============================

func (s *Service) ListResearchMemberships(ctx context.Context, personID string) ([]domain.ResearchMembership, error) {
	return s.newRepo(s.pool).ListResearchMembershipsByPerson(ctx, personID)
}

func (s *Service) CreateResearchMembership(ctx context.Context, personID string, in domain.ResearchMembershipInput) (domain.ResearchMembership, error) {
	if err := in.Validate(); err != nil {
		return domain.ResearchMembership{}, err
	}
	var out domain.ResearchMembership
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertResearchMembership(ctx, personID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.research-membership.create", v.ID, v)
	})
	return out, err
}

func (s *Service) UpdateResearchMembership(ctx context.Context, personID, id string, in domain.ResearchMembershipInput) (domain.ResearchMembership, error) {
	var out domain.ResearchMembership
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateResearchMembership(ctx, personID, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.research-membership.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteResearchMembership(ctx context.Context, personID, id string) error {
	return s.refDelete(ctx, "education.research-membership.delete", id, func(r domain.Repository) (int64, error) {
		return r.SoftDeleteResearchMembership(ctx, personID, id)
	})
}

// ============================ person: grant holdings ============================

func (s *Service) ListGrantHoldings(ctx context.Context, personID string) ([]domain.GrantHolding, error) {
	return s.newRepo(s.pool).ListGrantHoldingsByPerson(ctx, personID)
}

func (s *Service) CreateGrantHolding(ctx context.Context, personID string, in domain.GrantHoldingInput) (domain.GrantHolding, error) {
	if err := in.Validate(); err != nil {
		return domain.GrantHolding{}, err
	}
	var out domain.GrantHolding
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertGrantHolding(ctx, personID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.grant-holding.create", v.ID, v)
	})
	return out, err
}

func (s *Service) UpdateGrantHolding(ctx context.Context, personID, id string, in domain.GrantHoldingInput) (domain.GrantHolding, error) {
	var out domain.GrantHolding
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateGrantHolding(ctx, personID, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.grant-holding.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteGrantHolding(ctx context.Context, personID, id string) error {
	return s.refDelete(ctx, "education.grant-holding.delete", id, func(r domain.Repository) (int64, error) {
		return r.SoftDeleteGrantHolding(ctx, personID, id)
	})
}

// ============================ person: governance memberships ============================

func (s *Service) ListGovernanceMemberships(ctx context.Context, personID string) ([]domain.GovernanceMembership, error) {
	return s.newRepo(s.pool).ListGovernanceMembershipsByPerson(ctx, personID)
}

func (s *Service) CreateGovernanceMembership(ctx context.Context, personID string, in domain.GovernanceMembershipInput) (domain.GovernanceMembership, error) {
	if err := in.Validate(); err != nil {
		return domain.GovernanceMembership{}, err
	}
	var out domain.GovernanceMembership
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertGovernanceMembership(ctx, personID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.governance-membership.create", v.ID, v)
	})
	return out, err
}

func (s *Service) UpdateGovernanceMembership(ctx context.Context, personID, id string, in domain.GovernanceMembershipInput) (domain.GovernanceMembership, error) {
	var out domain.GovernanceMembership
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateGovernanceMembership(ctx, personID, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.governance-membership.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteGovernanceMembership(ctx context.Context, personID, id string) error {
	return s.refDelete(ctx, "education.governance-membership.delete", id, func(r domain.Repository) (int64, error) {
		return r.SoftDeleteGovernanceMembership(ctx, personID, id)
	})
}

// ============================ person: qualification awards ============================

func (s *Service) ListQualificationAwards(ctx context.Context, personID string) ([]domain.QualificationAward, error) {
	return s.newRepo(s.pool).ListQualificationAwardsByPerson(ctx, personID)
}

func (s *Service) CreateQualificationAward(ctx context.Context, personID string, in domain.QualificationAwardInput) (domain.QualificationAward, error) {
	if err := in.Validate(); err != nil {
		return domain.QualificationAward{}, err
	}
	var out domain.QualificationAward
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertQualificationAward(ctx, personID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.qualification-award.create", v.ID, v)
	})
	return out, err
}

func (s *Service) UpdateQualificationAward(ctx context.Context, personID, id string, in domain.QualificationAwardInput) (domain.QualificationAward, error) {
	var out domain.QualificationAward
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateQualificationAward(ctx, personID, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.qualification-award.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteQualificationAward(ctx context.Context, personID, id string) error {
	return s.refDelete(ctx, "education.qualification-award.delete", id, func(r domain.Repository) (int64, error) {
		return r.SoftDeleteQualificationAward(ctx, personID, id)
	})
}

// ============================ person: scholarship awards ============================

func (s *Service) ListScholarshipAwards(ctx context.Context, personID string) ([]domain.ScholarshipAward, error) {
	return s.newRepo(s.pool).ListScholarshipAwardsByPerson(ctx, personID)
}

func (s *Service) CreateScholarshipAward(ctx context.Context, personID string, in domain.ScholarshipAwardInput) (domain.ScholarshipAward, error) {
	if err := in.Validate(); err != nil {
		return domain.ScholarshipAward{}, err
	}
	var out domain.ScholarshipAward
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).InsertScholarshipAward(ctx, personID, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.scholarship-award.create", v.ID, v)
	})
	return out, err
}

func (s *Service) UpdateScholarshipAward(ctx context.Context, personID, id string, in domain.ScholarshipAwardInput) (domain.ScholarshipAward, error) {
	var out domain.ScholarshipAward
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateScholarshipAward(ctx, personID, id, in)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "education.scholarship-award.update", id, v)
	})
	return out, err
}

func (s *Service) DeleteScholarshipAward(ctx context.Context, personID, id string) error {
	return s.refDelete(ctx, "education.scholarship-award.delete", id, func(r domain.Repository) (int64, error) {
		return r.SoftDeleteScholarshipAward(ctx, personID, id)
	})
}
