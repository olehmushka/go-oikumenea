// Package transport implements the membership module's generated Conjure MembershipService
// interface: it translates the wire contract to/from the application service, assembles localized
// position `title` maps via the localization service (cross-module query — overview.md), and maps
// domain errors to Conjure SerializableErrors (D-Conjure). Generated code in internal/conjure is
// never hand-edited.
//
// A position `title` is a translatable label returned as a `locale -> text` map (assembled from the
// i18n store under the "title" field); a membership references its person/unit/position by RID
// (clients resolve names via the owning services).
//
// Authorization (M7): unit-keyed endpoints (create/list positions, create membership, list members)
// gate on their `position.*`/`membership.*` permission AT the unit via the PEP; id-keyed endpoints
// (get/update/abolish a position, fill/end a membership, list a person's memberships) have no unit in
// the request, so they use the coarse "holds the permission anywhere" form pending the load-then-check
// + shadow gate tightening (a documented follow-up, cleanest once M8 supplies a real subject). The
// bearer token carries the acting subject (interim: token == person RID; see
// internal/authorization/pep).
package transport

import (
	"context"
	"errors"
	"time"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	membershipapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/membership"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/membership/application"
	"github.com/olegamysk/go-oikumenea/internal/membership/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// titleField is the i18n store field a position's translatable title lives under (D-Code: code vs
// translatable label).
const titleField = "title"

// positionEntity is the localization entity-type key for position titles.
const positionEntity = "position"

// Service adapts *application.Service to the generated membershipapi.MembershipService interface. It
// holds the localization service to assemble the `locale -> text` position title maps responses
// return.
type Service struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the membership application service, the localization
// service (for title-map assembly), and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, loc: loc, pep: enforcer}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ membershipapi.MembershipService = Service{}

// ---------------------------------------------------------------- positions

func (s Service) CreatePosition(ctx context.Context, token bearertoken.Token, unitID string, req membershipapi.CreatePositionRequest) (membershipapi.Position, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermPositionCreate), unitID); err != nil {
		return membershipapi.Position{}, err
	}
	created, err := s.app.CreatePosition(ctx, domain.Position{
		UnitID:         unitID,
		Code:           req.Code,
		Title:          req.Title,
		RequiredRankID: derefOr(req.RequiredRankId, ""),
		SortOrder:      req.SortOrder,
	})
	if err != nil {
		return membershipapi.Position{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	return s.positionToAPI(ctx, created)
}

func (s Service) ListPositions(ctx context.Context, token bearertoken.Token, unitID string, state *string, pageSize *int, pageToken *string) (membershipapi.PositionPage, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermPositionRead), unitID); err != nil {
		return membershipapi.PositionPage{}, err
	}
	filter, err := parseFilter(state)
	if err != nil {
		return membershipapi.PositionPage{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	page, err := s.app.ListPositions(ctx, unitID, filter, derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return membershipapi.PositionPage{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	positions, err := s.positionsToAPI(ctx, page.Positions)
	if err != nil {
		return membershipapi.PositionPage{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	return membershipapi.PositionPage{Positions: positions, NextPageToken: tokenPtr(page.NextPageToken)}, nil
}

func (s Service) GetPosition(ctx context.Context, token bearertoken.Token, positionID string) (membershipapi.Position, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPositionRead)); err != nil {
		return membershipapi.Position{}, err
	}
	p, err := s.app.GetPosition(ctx, positionID)
	if err != nil {
		return membershipapi.Position{}, s.mapError(ctx, err, errCtx{positionID: positionID})
	}
	return s.positionToAPI(ctx, p)
}

func (s Service) UpdatePosition(ctx context.Context, token bearertoken.Token, positionID string, req membershipapi.UpdatePositionRequest) (membershipapi.Position, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPositionUpdate)); err != nil {
		return membershipapi.Position{}, err
	}
	updated, err := s.app.UpdatePosition(ctx, positionID, domain.PositionPatch{
		Title:          req.Title,
		RequiredRankID: req.RequiredRankId,
		SortOrder:      req.SortOrder,
	})
	if err != nil {
		return membershipapi.Position{}, s.mapError(ctx, err, errCtx{positionID: positionID})
	}
	return s.positionToAPI(ctx, updated)
}

func (s Service) AbolishPosition(ctx context.Context, token bearertoken.Token, positionID string) (membershipapi.Position, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPositionUpdate)); err != nil {
		return membershipapi.Position{}, err
	}
	abolished, err := s.app.AbolishPosition(ctx, positionID)
	if err != nil {
		return membershipapi.Position{}, s.mapError(ctx, err, errCtx{positionID: positionID})
	}
	return s.positionToAPI(ctx, abolished)
}

// ---------------------------------------------------------------- memberships

func (s Service) CreateMembership(ctx context.Context, token bearertoken.Token, req membershipapi.CreateMembershipRequest) (membershipapi.Membership, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermMembershipCreate), req.UnitId); err != nil {
		return membershipapi.Membership{}, err
	}
	created, err := s.app.CreateMembership(ctx, domain.Membership{
		PersonID:      req.PersonId,
		UnitID:        req.UnitId,
		PositionID:    derefOr(req.PositionId, ""),
		OrderItemID:   derefOr(req.OrderItemId, ""),
		EffectiveFrom: dtVal(req.EffectiveFrom),
	})
	if err != nil {
		return membershipapi.Membership{}, s.mapError(ctx, err, errCtx{positionID: derefOr(req.PositionId, "")})
	}
	return toAPIMembership(created), nil
}

func (s Service) FillPosition(ctx context.Context, token bearertoken.Token, positionID string, req membershipapi.FillPositionRequest) (membershipapi.Membership, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermMembershipCreate)); err != nil {
		return membershipapi.Membership{}, err
	}
	created, err := s.app.FillPosition(ctx, positionID, req.PersonId, derefOr(req.OrderItemId, ""), dtVal(req.EffectiveFrom))
	if err != nil {
		return membershipapi.Membership{}, s.mapError(ctx, err, errCtx{positionID: positionID})
	}
	return toAPIMembership(created), nil
}

func (s Service) EndMembership(ctx context.Context, token bearertoken.Token, membershipID string, req membershipapi.EndMembershipRequest) (membershipapi.Membership, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermMembershipUpdate)); err != nil {
		return membershipapi.Membership{}, err
	}
	ended, err := s.app.EndMembership(ctx, membershipID, derefOr(req.OrderItemId, ""), dtVal(req.EffectiveTo))
	if err != nil {
		return membershipapi.Membership{}, s.mapError(ctx, err, errCtx{membershipID: membershipID})
	}
	return toAPIMembership(ended), nil
}

func (s Service) ListMembers(ctx context.Context, token bearertoken.Token, unitID string, pageSize *int, pageToken *string) (membershipapi.MembershipPage, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermMembershipRead), unitID); err != nil {
		return membershipapi.MembershipPage{}, err
	}
	page, err := s.app.ListMembers(ctx, unitID, derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return membershipapi.MembershipPage{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	return toAPIMembershipPage(page), nil
}

func (s Service) ListPersonMemberships(ctx context.Context, token bearertoken.Token, personID string, pageSize *int, pageToken *string) (membershipapi.MembershipPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermMembershipRead)); err != nil {
		return membershipapi.MembershipPage{}, err
	}
	page, err := s.app.ListPersonMemberships(ctx, personID, derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return membershipapi.MembershipPage{}, s.mapError(ctx, err, errCtx{})
	}
	return toAPIMembershipPage(page), nil
}

// ---------------------------------------------------------------- response assembly

func (s Service) positionToAPI(ctx context.Context, p domain.Position) (membershipapi.Position, error) {
	titles, err := s.loc.LabelsByID(ctx, positionEntity, titleField, map[string]string{p.ID: p.Title})
	if err != nil {
		return membershipapi.Position{}, err
	}
	return toAPIPosition(p, titles[p.ID]), nil
}

func (s Service) positionsToAPI(ctx context.Context, positions []domain.Position) ([]membershipapi.Position, error) {
	defaults := make(map[string]string, len(positions))
	for _, p := range positions {
		defaults[p.ID] = p.Title
	}
	titles, err := s.loc.LabelsByID(ctx, positionEntity, titleField, defaults)
	if err != nil {
		return nil, err
	}
	out := make([]membershipapi.Position, 0, len(positions))
	for _, p := range positions {
		out = append(out, toAPIPosition(p, titles[p.ID]))
	}
	return out, nil
}

func toAPIPosition(p domain.Position, title map[string]string) membershipapi.Position {
	pos := membershipapi.Position{
		Id:             p.ID,
		UnitId:         p.UnitID,
		Code:           p.Code,
		Title:          title,
		RequiredRankId: strPtrOrNil(p.RequiredRankID),
		Status:         string(p.Status),
		SortOrder:      p.SortOrder,
		CreatedAt:      datetime.DateTime(p.CreatedAt),
		UpdatedAt:      datetime.DateTime(p.UpdatedAt),
	}
	if p.Holder != nil {
		h := toAPIMembership(*p.Holder)
		pos.Holder = &h
	}
	return pos
}

func toAPIMembership(m domain.Membership) membershipapi.Membership {
	return membershipapi.Membership{
		Id:            m.ID,
		PersonId:      m.PersonID,
		UnitId:        m.UnitID,
		PositionId:    strPtrOrNil(m.PositionID),
		OrderItemId:   strPtrOrNil(m.OrderItemID),
		Status:        string(m.Status),
		EffectiveFrom: datetime.DateTime(m.EffectiveFrom),
		EffectiveTo:   dtPtr(m.EffectiveTo),
		CreatedAt:     datetime.DateTime(m.CreatedAt),
		UpdatedAt:     datetime.DateTime(m.UpdatedAt),
	}
}

func toAPIMembershipPage(page application.MembershipPage) membershipapi.MembershipPage {
	ms := make([]membershipapi.Membership, 0, len(page.Memberships))
	for _, m := range page.Memberships {
		ms = append(ms, toAPIMembership(m))
	}
	return membershipapi.MembershipPage{Memberships: ms, NextPageToken: tokenPtr(page.NextPageToken)}
}

// ---------------------------------------------------------------- error mapping

// errCtx carries the identifiers an endpoint can name in a Conjure error (only the relevant fields
// are set per call).
type errCtx struct {
	unitID       string
	positionID   string
	membershipID string
}

// mapError translates domain/application errors into the Conjure SerializableError contract.
func (s Service) mapError(ctx context.Context, err error, c errCtx) error {
	switch {
	case errors.Is(err, domain.ErrPositionNotFound):
		return membershipapi.NewPositionNotFound(c.positionID)
	case errors.Is(err, domain.ErrPositionCodeConflict):
		return membershipapi.NewPositionConflict("a position with this code already exists in the unit")
	case errors.Is(err, domain.ErrPositionInUse):
		return membershipapi.NewPositionInUse(c.positionID)
	case errors.Is(err, domain.ErrPositionInvalid):
		return membershipapi.NewPositionInvalid(err.Error())
	case errors.Is(err, domain.ErrMembershipNotFound):
		return membershipapi.NewMembershipNotFound(c.membershipID)
	case errors.Is(err, domain.ErrPositionAlreadyFilled):
		return membershipapi.NewPositionAlreadyFilled(c.positionID)
	case errors.Is(err, domain.ErrMembershipConflict):
		return membershipapi.NewMembershipConflict("an active membership for this person and unit already exists")
	case errors.Is(err, domain.ErrMembershipLifecycle):
		return membershipapi.NewMembershipLifecycleConflict(err.Error())
	case errors.Is(err, domain.ErrUnknownUnit):
		return membershipapi.NewMembershipInvalid("unit does not exist")
	case errors.Is(err, domain.ErrUnknownPerson):
		return membershipapi.NewMembershipInvalid("person does not exist")
	case errors.Is(err, domain.ErrUnknownRank):
		return membershipapi.NewPositionInvalid("rank does not exist")
	case errors.Is(err, domain.ErrUnknownPosition):
		return membershipapi.NewMembershipInvalid("position does not exist")
	case errors.Is(err, domain.ErrPositionUnitMismatch):
		return membershipapi.NewMembershipInvalid("position does not belong to the unit")
	case errors.Is(err, domain.ErrMembershipInvalid):
		return membershipapi.NewMembershipInvalid(err.Error())
	default:
		return werror.WrapWithContextParams(ctx, err, "membership request failed")
	}
}

// ---------------------------------------------------------------- value helpers

// parseFilter validates the optional position state filter (vacant | filled); empty lists all active.
func parseFilter(state *string) (domain.PositionFilter, error) {
	switch derefOr(state, "") {
	case "":
		return domain.FilterAll, nil
	case string(domain.FilterVacant):
		return domain.FilterVacant, nil
	case string(domain.FilterFilled):
		return domain.FilterFilled, nil
	default:
		return domain.FilterAll, errors.Join(domain.ErrPositionInvalid, errors.New("state must be one of vacant|filled"))
	}
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func tokenPtr(token string) *string {
	if token == "" {
		return nil
	}
	return &token
}

func derefOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

// dtVal converts an optional Conjure datetime to a time.Time; nil -> the zero time (the application
// defaults effective_from/to to now()).
func dtVal(d *datetime.DateTime) time.Time {
	if d == nil {
		return time.Time{}
	}
	return time.Time(*d)
}

func dtPtr(t *time.Time) *datetime.DateTime {
	if t == nil {
		return nil
	}
	d := datetime.DateTime(*t)
	return &d
}
