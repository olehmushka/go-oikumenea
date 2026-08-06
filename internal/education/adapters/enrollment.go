// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/education/adapters/educationsql"
	"github.com/olegamysk/go-oikumenea/internal/education/domain"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// The enrollment register's browse + dashboard halves (M58 ticket 7 / D-ObjectFacets).
//
// TWO arms, {instance-admin, holder-scoped} — not institution's four, because an enrollment has no
// name to search (every part of it is a RID, a code or a date) and no visibility bit of its own. The
// scope it does have comes from its HOLDER (D-PersonReadScope), which is the document arrangement
// rather than any other M58 type's.

// enrollmentFacetArgsT is the enrollment facet block bound to pgx wire types — ONE mapping used by
// the admin list, BOTH scoped list plan shapes and BOTH aggregate arms, so no two paths can bind a
// facet differently. Every field is a nullable pgx type: an unset filter is a SQL NULL the query's
// `sqlc.narg('x')::type IS NULL OR …` guard short-circuits, never a sentinel (R-21).
type enrollmentFacetArgsT struct {
	institutionID     pgtype.Text
	programID         pgtype.Text
	unitID            pgtype.Text
	groupID           pgtype.Text
	degreeLevelID     pgtype.Text
	status            pgtype.Text
	effectiveFromFrom pgtype.Date
	effectiveFromTo   pgtype.Date
}

func enrollmentFacetArgs(f domain.EnrollmentFilter) enrollmentFacetArgsT {
	return enrollmentFacetArgsT{
		institutionID:     textPtr(f.InstitutionID),
		programID:         textPtr(f.ProgramID),
		unitID:            textPtr(f.UnitID),
		groupID:           textPtr(f.GroupID),
		degreeLevelID:     textPtr(f.DegreeLevelID),
		status:            textPtr(f.Status),
		effectiveFromFrom: datePtr(f.EffectiveFromFrom),
		effectiveFromTo:   datePtr(f.EffectiveFromTo),
	}
}

// ListEnrollments pages the register under the facet filter block. subjectPersonID empty is the
// INSTANCE-ADMIN arm; otherwise the rows are trimmed to the enrollments of holders the subject may
// read, in SQL, and the scoped path picks between two plan shapes on reach cardinality (see
// listReachIsDense and migration 0017).
func (r *Repository) ListEnrollments(ctx context.Context, subjectPersonID string, f domain.EnrollmentFilter, after string, limit int) ([]domain.Enrollment, error) {
	fa := enrollmentFacetArgs(f)
	if subjectPersonID == "" {
		rows, err := r.q.ListEnrollmentsPage(ctx, educationsql.ListEnrollmentsPageParams{
			After:             after,
			InstitutionID:     fa.institutionID,
			ProgramID:         fa.programID,
			UnitID:            fa.unitID,
			GroupID:           fa.groupID,
			DegreeLevelID:     fa.degreeLevelID,
			Status:            fa.status,
			EffectiveFromFrom: fa.effectiveFromFrom,
			EffectiveFromTo:   fa.effectiveFromTo,
			Lim:               int32(limit),
		})
		if err != nil {
			return nil, err
		}
		return enrollmentsFrom(len(rows), func(i int) educationsql.OikumeneaPersonEducationEnrollment { return rows[i] }), nil
	}
	dense, err := r.listReachIsDense(ctx, subjectPersonID)
	if err != nil {
		return nil, err
	}
	if dense {
		rows, err := r.q.ListEnrollmentsPageForSubjectDense(ctx, educationsql.ListEnrollmentsPageForSubjectDenseParams{
			After:             after,
			SubjectPersonID:   subjectPersonID,
			InstitutionID:     fa.institutionID,
			ProgramID:         fa.programID,
			UnitID:            fa.unitID,
			GroupID:           fa.groupID,
			DegreeLevelID:     fa.degreeLevelID,
			Status:            fa.status,
			EffectiveFromFrom: fa.effectiveFromFrom,
			EffectiveFromTo:   fa.effectiveFromTo,
			Lim:               int32(limit),
		})
		if err != nil {
			return nil, err
		}
		return enrollmentsFrom(len(rows), func(i int) educationsql.OikumeneaPersonEducationEnrollment { return rows[i] }), nil
	}
	rows, err := r.q.ListEnrollmentsPageForSubject(ctx, educationsql.ListEnrollmentsPageForSubjectParams{
		After:             after,
		SubjectPersonID:   subjectPersonID,
		InstitutionID:     fa.institutionID,
		ProgramID:         fa.programID,
		UnitID:            fa.unitID,
		GroupID:           fa.groupID,
		DegreeLevelID:     fa.degreeLevelID,
		Status:            fa.status,
		EffectiveFromFrom: fa.effectiveFromFrom,
		EffectiveFromTo:   fa.effectiveFromTo,
		Lim:               int32(limit),
	})
	if err != nil {
		return nil, err
	}
	return enrollmentsFrom(len(rows), func(i int) educationsql.OikumeneaPersonEducationEnrollment { return rows[i] }), nil
}

// denseReachThreshold and listReachIsDense mirror document's dispatch, which is the closest shape:
// the same holder semi-join over the same reach functions. Duplicated as a THRESHOLD rather than
// shared, because the reach ALGEBRA — the part that must never diverge — lives in one place, the SQL
// functions migration 0017 defines; this is only the cardinality at which the two plans cross over.
const denseReachThreshold = 1000

func (r *Repository) listReachIsDense(ctx context.Context, subjectPersonID string) (bool, error) {
	n, err := r.q.CountReadableUnitsForDispatch(ctx, educationsql.CountReadableUnitsForDispatchParams{
		SubjectPersonID: subjectPersonID, Cap: denseReachThreshold + 1,
	})
	if err != nil {
		return false, err
	}
	return n > denseReachThreshold, nil
}

// EnrollmentStats answers the whole dashboard in one statement, over the same candidate set the list
// pages under the same filter.
//
// subjectPersonID empty means the INSTANCE-ADMIN arm, which carries no scope predicate at all.
// Otherwise the holder semi-join is folded into the candidate CTE — right for a count, where trimming
// after the fact would silently disagree with the caller's own paging.
func (r *Repository) EnrollmentStats(ctx context.Context, subjectPersonID string, f domain.EnrollmentFilter, sel stats.Selection) ([]stats.Group, error) {
	fa := enrollmentFacetArgs(f)
	w := enrollmentStatsWants(sel)
	if subjectPersonID != "" {
		rows, err := r.q.EnrollmentStatsForSubject(ctx, educationsql.EnrollmentStatsForSubjectParams{
			SubjectPersonID:   subjectPersonID,
			InstitutionID:     fa.institutionID,
			ProgramID:         fa.programID,
			UnitID:            fa.unitID,
			GroupID:           fa.groupID,
			DegreeLevelID:     fa.degreeLevelID,
			Status:            fa.status,
			EffectiveFromFrom: fa.effectiveFromFrom,
			EffectiveFromTo:   fa.effectiveFromTo,
			TopN:              int32(sel.TopN()),
			WantInstitutionID: w.institutionID,
			WantProgramID:     w.programID,
			WantUnitID:        w.unitID,
			WantGroupID:       w.groupID,
			WantDegreeLevelID: w.degreeLevelID,
			WantStatus:        w.status,
			WantEffectiveFrom: w.effectiveFrom,
		})
		if err != nil {
			return nil, err
		}
		return enrollmentStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64, pgtype.Int8) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N, rows[i].Ord
		}), nil
	}
	rows, err := r.q.EnrollmentStats(ctx, educationsql.EnrollmentStatsParams{
		InstitutionID:     fa.institutionID,
		ProgramID:         fa.programID,
		UnitID:            fa.unitID,
		GroupID:           fa.groupID,
		DegreeLevelID:     fa.degreeLevelID,
		Status:            fa.status,
		EffectiveFromFrom: fa.effectiveFromFrom,
		EffectiveFromTo:   fa.effectiveFromTo,
		TopN:              int32(sel.TopN()),
		WantInstitutionID: w.institutionID,
		WantProgramID:     w.programID,
		WantUnitID:        w.unitID,
		WantGroupID:       w.groupID,
		WantDegreeLevelID: w.degreeLevelID,
		WantStatus:        w.status,
		WantEffectiveFrom: w.effectiveFrom,
	})
	if err != nil {
		return nil, err
	}
	return enrollmentStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64, pgtype.Int8) {
		return rows[i].Facet, rows[i].Bucket, rows[i].N, rows[i].Ord
	}), nil
}

// enrollmentStatsWants projects a selection onto the per-branch flags. An unselected facet's branch is
// a one-time false filter the planner skips, so it is never grouped.
type enrollmentStatsWantFlags struct {
	institutionID, programID, unitID, groupID, degreeLevelID, status, effectiveFrom bool
}

func enrollmentStatsWants(sel stats.Selection) enrollmentStatsWantFlags {
	return enrollmentStatsWantFlags{
		institutionID: sel.Wants("institutionId"),
		programID:     sel.Wants("programId"),
		unitID:        sel.Wants("unitId"),
		groupID:       sel.Wants("groupId"),
		degreeLevelID: sel.Wants("degreeLevelId"),
		status:        sel.Wants("status"),
		effectiveFrom: sel.Wants("effectiveFrom"),
	}
}

// enrollmentStatsGroups maps the raw aggregate rows; a NULL bucket stays NULL (the (unknown) bucket).
//
// Unlike institutionStatsGroups it CARRIES THE ORD through, because degreeLevelId is a catalog-ordered
// facet (facet.StrategyCatalog): the ISCED level travels in that column and is what pkg/stats sorts
// the scale by. Dropping it here would leave the chart looking correct and ordered by frequency —
// exactly the failure the strategy exists to prevent, and invisible in the response's shape.
func enrollmentStatsGroups(n int, at func(int) (string, pgtype.Text, int64, pgtype.Int8)) []stats.Group {
	out := make([]stats.Group, 0, n)
	for i := 0; i < n; i++ {
		facetKey, bucket, count, ord := at(i)
		g := stats.Group{Facet: facetKey, Count: count}
		if bucket.Valid {
			k := bucket.String
			g.Key = &k
		}
		if ord.Valid {
			o := ord.Int64
			g.Ord = &o
		}
		out = append(out, g)
	}
	return out
}

// enrollmentsFrom maps the generated row types onto the domain. The three list plan shapes return
// three identical generated types, so the accessor is passed in rather than the slice — one mapping,
// three callers (the institutionStatsGroups arrangement).
func enrollmentsFrom(n int, at func(int) educationsql.OikumeneaPersonEducationEnrollment) []domain.Enrollment {
	out := make([]domain.Enrollment, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, toEnrollment(at(i)))
	}
	return out
}
