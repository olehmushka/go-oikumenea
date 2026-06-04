// Package adapters implements the authorization domain ports against infrastructure: the pgx/sqlc
// repository over the oikumenea.authz_* tables. It depends on the database, never the reverse
// (overview.md). Generated sqlc code lives in the authzsql subpackage and is never hand-edited.
package adapters

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/authorization/adapters/authzsql"
	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository, bound to a single db.DBTX —
// the pool for reads, or a caller-supplied transaction so a write and its audit row commit together
// (D-Audit).
type Repository struct {
	q *authzsql.Queries
}

// NewRepository binds a repository to the given command surface (pool or tx).
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: authzsql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*Repository)(nil)

// ---------------------------------------------------------------- roles

func (r *Repository) InsertRole(ctx context.Context, role domain.Role) (domain.Role, error) {
	row, err := r.q.InsertRole(ctx, authzsql.InsertRoleParams{
		Code:        role.Code,
		Name:        role.Name,
		Description: text(role.Description),
		IsBase:      role.IsBase,
	})
	if err != nil {
		return domain.Role{}, mapWriteErr(err)
	}
	return domain.Role{
		ID: row.ID, Code: row.Code, Name: row.Name, Description: row.Description.String,
		IsBase: row.IsBase, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *Repository) GetRole(ctx context.Context, id string) (domain.Role, error) {
	row, err := r.q.GetRole(ctx, id)
	if err != nil {
		return domain.Role{}, mapReadErr(err, domain.ErrRoleNotFound)
	}
	out := domain.Role{
		ID: row.ID, Code: row.Code, Name: row.Name, Description: row.Description.String,
		IsBase: row.IsBase, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
	return r.withPerms(ctx, out)
}

func (r *Repository) GetRoleByCode(ctx context.Context, code string) (domain.Role, error) {
	row, err := r.q.GetRoleByCode(ctx, code)
	if err != nil {
		return domain.Role{}, mapReadErr(err, domain.ErrRoleNotFound)
	}
	out := domain.Role{
		ID: row.ID, Code: row.Code, Name: row.Name, Description: row.Description.String,
		IsBase: row.IsBase, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
	return r.withPerms(ctx, out)
}

func (r *Repository) ListRoles(ctx context.Context, after string, limit int) ([]domain.Role, error) {
	rows, err := r.q.ListRoles(ctx, authzsql.ListRolesParams{After: after, Lim: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Role, 0, len(rows))
	for _, row := range rows {
		role := domain.Role{
			ID: row.ID, Code: row.Code, Name: row.Name, Description: row.Description.String,
			IsBase: row.IsBase, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		}
		role, err = r.withPerms(ctx, role)
		if err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	return out, nil
}

func (r *Repository) UpdateRole(ctx context.Context, id string, patch domain.RolePatch) (domain.Role, error) {
	row, err := r.q.UpdateRole(ctx, authzsql.UpdateRoleParams{
		ID:          id,
		Name:        textPtr(patch.Name),
		Description: textPtr(patch.Description),
	})
	if err != nil {
		return domain.Role{}, mapReadErr(err, domain.ErrRoleNotFound)
	}
	out := domain.Role{
		ID: row.ID, Code: row.Code, Name: row.Name, Description: row.Description.String,
		IsBase: row.IsBase, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
	return r.withPerms(ctx, out)
}

func (r *Repository) SoftDeleteRole(ctx context.Context, id string) error {
	return r.q.SoftDeleteRole(ctx, id)
}

func (r *Repository) RoleHasActiveAssignments(ctx context.Context, roleID string) (bool, error) {
	return r.q.RoleHasActiveAssignments(ctx, roleID)
}

func (r *Repository) ReplaceRolePermissions(ctx context.Context, roleID string, perms []domain.Permission) error {
	if err := r.q.DeleteRolePermissions(ctx, roleID); err != nil {
		return err
	}
	for _, p := range perms {
		if err := r.q.InsertRolePermission(ctx, authzsql.InsertRolePermissionParams{
			RoleID: roleID, PermissionCode: string(p),
		}); err != nil {
			return mapWriteErr(err)
		}
	}
	return nil
}

func (r *Repository) withPerms(ctx context.Context, role domain.Role) (domain.Role, error) {
	codes, err := r.q.GetRolePermissions(ctx, role.ID)
	if err != nil {
		return domain.Role{}, err
	}
	role.Permissions = make([]domain.Permission, 0, len(codes))
	for _, c := range codes {
		role.Permissions = append(role.Permissions, domain.Permission(c))
	}
	return role, nil
}

// ---------------------------------------------------------------- assignments

func (r *Repository) InsertAssignment(ctx context.Context, g domain.GrantInput, graphID string) (domain.Assignment, error) {
	row, err := r.q.InsertAssignment(ctx, authzsql.InsertAssignmentParams{
		SubjectPersonID: g.SubjectPersonID,
		RoleID:          g.RoleID,
		TargetUnitID:    g.TargetUnitID,
		Scope:           string(g.Scope),
		GraphID:         text(graphID),
		GrantedBy:       text(g.GrantedBy),
		ExpiresAt:       tsPtr(g.ExpiresAt),
	})
	if err != nil {
		return domain.Assignment{}, mapWriteErr(err)
	}
	return assignmentFrom(row), nil
}

func (r *Repository) GetAssignment(ctx context.Context, id string) (domain.Assignment, error) {
	row, err := r.q.GetAssignment(ctx, id)
	if err != nil {
		return domain.Assignment{}, mapReadErr(err, domain.ErrAssignmentNotFound)
	}
	return assignmentFrom(row), nil
}

func (r *Repository) RevokeAssignment(ctx context.Context, id, revokedBy string) (domain.Assignment, error) {
	row, err := r.q.RevokeAssignment(ctx, authzsql.RevokeAssignmentParams{ID: id, RevokedBy: text(revokedBy)})
	if err != nil {
		return domain.Assignment{}, mapReadErr(err, domain.ErrAssignmentNotFound)
	}
	return assignmentFrom(row), nil
}

func (r *Repository) ListAssignmentsBySubject(ctx context.Context, subjectPersonID, after string, limit int) ([]domain.Assignment, error) {
	rows, err := r.q.ListAssignmentsBySubject(ctx, authzsql.ListAssignmentsBySubjectParams{
		SubjectPersonID: subjectPersonID, After: after, Lim: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	return assignmentsFrom(rows), nil
}

func (r *Repository) ListAssignmentsByUnit(ctx context.Context, targetUnitID, after string, limit int) ([]domain.Assignment, error) {
	rows, err := r.q.ListAssignmentsByUnit(ctx, authzsql.ListAssignmentsByUnitParams{
		TargetUnitID: targetUnitID, After: after, Lim: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	return assignmentsFrom(rows), nil
}

func (r *Repository) ActiveGrantsForSubject(ctx context.Context, subjectPersonID string) ([]domain.ActiveGrant, error) {
	rows, err := r.q.ActiveGrantsForSubject(ctx, subjectPersonID)
	if err != nil {
		return nil, err
	}
	// Rows are ordered by assignment id; group consecutive rows into one ActiveGrant.
	byID := make(map[string]*domain.ActiveGrant)
	order := make([]string, 0)
	for _, row := range rows {
		g, ok := byID[row.ID]
		if !ok {
			g = &domain.ActiveGrant{
				AssignmentID: row.ID,
				RoleID:       row.RoleID,
				RoleCode:     row.RoleCode,
				TargetUnitID: row.TargetUnitID,
				Scope:        domain.Scope(row.Scope),
				GraphID:      row.GraphID.String,
				GraphCode:    row.GraphCode.String,
				Perms:        map[domain.Permission]struct{}{},
			}
			byID[row.ID] = g
			order = append(order, row.ID)
		}
		g.Perms[domain.Permission(row.PermissionCode)] = struct{}{}
	}
	out := make([]domain.ActiveGrant, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

// ---------------------------------------------------------------- instance admins

func (r *Repository) InsertInstanceAdmin(ctx context.Context, personID, grantedBy string) (domain.InstanceAdmin, error) {
	row, err := r.q.InsertInstanceAdmin(ctx, authzsql.InsertInstanceAdminParams{
		PersonID: personID, GrantedBy: text(grantedBy),
	})
	if err != nil {
		return domain.InstanceAdmin{}, mapWriteErr(err)
	}
	return instanceAdminFrom(row), nil
}

func (r *Repository) GetInstanceAdmin(ctx context.Context, id string) (domain.InstanceAdmin, error) {
	row, err := r.q.GetInstanceAdmin(ctx, id)
	if err != nil {
		return domain.InstanceAdmin{}, mapReadErr(err, domain.ErrInstanceAdminNotFound)
	}
	return instanceAdminFrom(row), nil
}

func (r *Repository) RevokeInstanceAdmin(ctx context.Context, id, revokedBy string) (domain.InstanceAdmin, error) {
	row, err := r.q.RevokeInstanceAdmin(ctx, authzsql.RevokeInstanceAdminParams{ID: id, RevokedBy: text(revokedBy)})
	if err != nil {
		return domain.InstanceAdmin{}, mapReadErr(err, domain.ErrInstanceAdminNotFound)
	}
	return instanceAdminFrom(row), nil
}

func (r *Repository) IsActiveInstanceAdmin(ctx context.Context, personID string) (bool, error) {
	return r.q.IsActiveInstanceAdmin(ctx, personID)
}

// HasActiveInstanceAdmin reports whether ANY active instance admin exists — the idempotency gate for
// the first-admin bootstrap (D-Bootstrap).
func (r *Repository) HasActiveInstanceAdmin(ctx context.Context) (bool, error) {
	return r.q.HasActiveInstanceAdmin(ctx)
}

// ---------------------------------------------------------------- conversions

func assignmentFrom(a authzsql.OikumeneaAuthzRoleAssignment) domain.Assignment {
	return domain.Assignment{
		ID:              a.ID,
		SubjectPersonID: a.SubjectPersonID,
		RoleID:          a.RoleID,
		TargetUnitID:    a.TargetUnitID,
		Scope:           domain.Scope(a.Scope),
		GraphID:         a.GraphID.String,
		GrantedBy:       a.GrantedBy.String,
		GrantedAt:       a.GrantedAt.Time,
		RevokedAt:       tsToPtr(a.RevokedAt),
		RevokedBy:       a.RevokedBy.String,
		ExpiresAt:       tsToPtr(a.ExpiresAt),
		CreatedAt:       a.CreatedAt.Time,
		UpdatedAt:       a.UpdatedAt.Time,
	}
}

func assignmentsFrom(rows []authzsql.OikumeneaAuthzRoleAssignment) []domain.Assignment {
	out := make([]domain.Assignment, 0, len(rows))
	for _, a := range rows {
		out = append(out, assignmentFrom(a))
	}
	return out
}

func instanceAdminFrom(a authzsql.OikumeneaAuthzInstanceAdmin) domain.InstanceAdmin {
	return domain.InstanceAdmin{
		ID:        a.ID,
		PersonID:  a.PersonID,
		GrantedBy: a.GrantedBy.String,
		GrantedAt: a.GrantedAt.Time,
		RevokedAt: tsToPtr(a.RevokedAt),
		RevokedBy: a.RevokedBy.String,
		CreatedAt: a.CreatedAt.Time,
		UpdatedAt: a.UpdatedAt.Time,
	}
}

// ---------------------------------------------------------------- error mapping

// mapReadErr maps a no-rows error to a domain not-found sentinel; other errors pass through.
func mapReadErr(err error, notFound error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	return err
}

// mapWriteErr translates Postgres constraint violations into domain sentinels by constraint name.
func mapWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	name := pgErr.ConstraintName
	switch pgErr.Code {
	case "23505": // unique_violation
		switch {
		case strings.Contains(name, "roles_code"):
			return domain.ErrRoleCodeConflict
		case strings.Contains(name, "assignments_active"):
			return domain.ErrAssignmentConflict
		case strings.Contains(name, "instance_admins_person"):
			return domain.ErrInstanceAdminConflict
		}
	case "23503": // foreign_key_violation
		switch {
		case strings.Contains(name, "subject_person"):
			return domain.ErrUnknownSubject
		case strings.Contains(name, "role_id"):
			return domain.ErrUnknownRole
		case strings.Contains(name, "target_unit"):
			return domain.ErrUnknownUnit
		case strings.Contains(name, "graph"):
			return domain.ErrUnknownGraph
		case strings.Contains(name, "person"): // instance-admin person_id fk
			return domain.ErrUnknownSubject
		}
	}
	return err
}

// ---------------------------------------------------------------- pgtype helpers

func text(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func textPtr(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func tsPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func tsToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
