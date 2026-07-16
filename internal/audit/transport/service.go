// Package transport implements the audit module's generated Conjure AuditService interface: it
// translates the wire contract to/from the application service and maps domain errors to Conjure
// SerializableErrors (overview.md; D-Conjure). Generated code in internal/conjure is never
// hand-edited.
//
// Authorization (M7): both reads require `audit.read`, enforced via the PEP. Audit reads are
// documented as unit-scoped exactly like person.read (D-Audit), but the audit log is not yet
// unit-keyed, so the gate is the coarse "holds audit.read somewhere (or instance admin)" form;
// per-unit audit filtering + the shadow gate over the closure are a follow-up (cleanest once M8
// supplies a real subject). The bearer token carries the acting subject (interim: token == person
// RID; see internal/authorization/pep).
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/audit/domain"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	auditapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/audit"
	"github.com/olegamysk/go-oikumenea/pkg/action"
	"github.com/olegamysk/go-oikumenea/pkg/rid"
	cerrors "github.com/palantir/conjure-go-runtime/v2/conjure-go-contract/errors"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// Service adapts *application.Service to the generated auditapi.AuditService interface.
type Service struct {
	app *application.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the audit application service and the PEP enforcer.
func NewService(app *application.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, pep: enforcer}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ auditapi.AuditService = Service{}

// Query implements the GET /audit endpoint.
func (s Service) Query(
	ctx context.Context,
	token bearertoken.Token,
	actorPersonID *string,
	actorType *auditapi.AuditActorType,
	targetType *string,
	targetID *string,
	unitID *string,
	action *string,
	outcome *auditapi.AuditOutcome,
	since *datetime.DateTime,
	until *datetime.DateTime,
	pageSize *int,
	pageToken *string,
) (auditapi.AuditEntryPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermAuditRead)); err != nil {
		return auditapi.AuditEntryPage{}, err
	}
	page, err := s.app.Query(ctx, application.QueryParams{
		ActorPersonID: actorPersonID,
		ActorType:     fromAPIActorType(actorType),
		TargetType:    targetType,
		TargetID:      targetID,
		UnitID:        unitID,
		Action:        action,
		Outcome:       fromAPIOutcome(outcome),
		Since:         fromAPITime(since),
		Until:         fromAPITime(until),
		PageSize:      deref(pageSize),
		PageToken:     deref(pageToken),
	})
	if err != nil {
		return auditapi.AuditEntryPage{}, mapError(ctx, err, "")
	}

	entries := make([]auditapi.AuditEntry, 0, len(page.Entries))
	for _, e := range page.Entries {
		entries = append(entries, toAPIEntry(e))
	}
	return auditapi.AuditEntryPage{
		Entries:       entries,
		NextPageToken: emptyToNil(page.NextPageToken),
	}, nil
}

// Get implements the GET /audit/{entryId} endpoint.
func (s Service) Get(ctx context.Context, token bearertoken.Token, entryID string) (auditapi.AuditEntry, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermAuditRead)); err != nil {
		return auditapi.AuditEntry{}, err
	}
	e, err := s.app.Get(ctx, entryID)
	if err != nil {
		return auditapi.AuditEntry{}, mapError(ctx, err, entryID)
	}
	return toAPIEntry(e), nil
}

// GetObjectHistory serves the reverse-chronological audit history of one object (D-Temporal tier b,
// review-2026-09 R-31): a projection of the audit ledger filtered to `target_id = rid`. Gated by
// `audit.read` like the rest of the audit surface (it is literally a convenience projection of
// `GET /audit?targetId=…`, so a stronger gate would be bypassable). The `before`/`after` change
// payloads are withheld (and `redacted=true`) unless the caller also holds the sensitive-reader
// capability, because a folded per-object timeline can surface pii up to the D-DataScope ceiling.
func (s Service) GetObjectHistory(
	ctx context.Context,
	token bearertoken.Token,
	rid string,
	pageSize *int,
	pageToken *string,
) (auditapi.ObjectHistory, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermAuditRead)); err != nil {
		return auditapi.ObjectHistory{}, err
	}
	reveal, err := s.holdsSensitiveRead(ctx, token)
	if err != nil {
		return auditapi.ObjectHistory{}, werror.WrapWithContextParams(ctx, err, "audit history authorization")
	}
	page, err := s.app.Query(ctx, application.QueryParams{
		TargetID:  &rid,
		PageSize:  deref(pageSize),
		PageToken: deref(pageToken),
	})
	if err != nil {
		return auditapi.ObjectHistory{}, mapError(ctx, err, "")
	}

	events := make([]auditapi.ObjectHistoryEvent, 0, len(page.Entries))
	for _, e := range page.Entries {
		ev := auditapi.ObjectHistoryEvent{
			At:            datetime.DateTime(e.CreatedAt),
			Action:        e.Action,
			ActorType:     toAPIActorType(e.ActorType),
			ActorPersonId: emptyToNil(e.ActorPersonID),
			Subsystem:     emptyToNil(e.Subsystem),
			TargetType:    e.TargetType,
			Outcome:       toAPIOutcome(e.Outcome),
			RequestId:     e.RequestID,
		}
		if reveal {
			ev.Before = toAPIJSON(e.Before)
			ev.After = toAPIJSON(e.After)
		}
		events = append(events, ev)
	}
	return auditapi.ObjectHistory{
		Rid:           rid,
		Events:        events,
		NextPageToken: emptyToNil(page.NextPageToken),
		Redacted:      !reveal,
	}, nil
}

// holdsSensitiveRead reports whether the token's subject can read pii:special person data — the bar
// for revealing before/after change payloads (D-Temporal R-31). It requires the full sensitive-read
// permission set (the same data a subject would need direct authority to read); an instance admin
// satisfies each probe via HoldsPermissionAnywhere's admin short-circuit.
func (s Service) holdsSensitiveRead(ctx context.Context, token bearertoken.Token) (bool, error) {
	for _, p := range authzdomain.SensitiveReadPermissions() {
		ok, err := s.pep.AllowedAnywhere(ctx, token, string(p))
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// mapError translates domain/application errors into the Conjure SerializableError contract.
func mapError(ctx context.Context, err error, entryID string) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return auditapi.NewAuditEntryNotFound(entryID)
	case errors.Is(err, application.ErrInvalidPageToken), errors.Is(err, domain.ErrInvalidEntry):
		return cerrors.WrapWithInvalidArgument(err)
	default:
		return werror.WrapWithContextParams(ctx, err, "audit read failed")
	}
}

func toAPIEntry(e domain.Entry) auditapi.AuditEntry {
	return auditapi.AuditEntry{
		Id:            e.ID,
		CreatedAt:     datetime.DateTime(e.CreatedAt),
		ActorType:     toAPIActorType(e.ActorType),
		ActorPersonId: emptyToNil(e.ActorPersonID),
		Subsystem:     emptyToNil(e.Subsystem),
		Action:        e.Action,
		TargetType:    e.TargetType,
		TargetId:      emptyToNil(e.TargetID),
		UnitId:        emptyToNil(e.UnitID),
		RequestId:     e.RequestID,
		Before:        toAPIJSON(e.Before),
		After:         toAPIJSON(e.After),
		Outcome:       toAPIOutcome(e.Outcome),
	}
}

func toAPIActorType(a domain.ActorType) auditapi.AuditActorType {
	return auditapi.New_AuditActorType(auditapi.AuditActorType_Value(strings.ToUpper(string(a))))
}

func toAPIOutcome(o domain.Outcome) auditapi.AuditOutcome {
	return auditapi.New_AuditOutcome(auditapi.AuditOutcome_Value(strings.ToUpper(string(o))))
}

func fromAPIActorType(a *auditapi.AuditActorType) *domain.ActorType {
	if a == nil {
		return nil
	}
	v := domain.ActorType(strings.ToLower(a.String()))
	return &v
}

func fromAPIOutcome(o *auditapi.AuditOutcome) *domain.Outcome {
	if o == nil {
		return nil
	}
	v := domain.Outcome(strings.ToLower(o.String()))
	return &v
}

func fromAPITime(dt *datetime.DateTime) *time.Time {
	if dt == nil {
		return nil
	}
	t := time.Time(*dt)
	return &t
}

// toAPIJSON decodes a stored JSONB payload into the Conjure `any` representation (a *interface{}).
// Invalid/empty JSON yields nil (omitted from the response) rather than a hard error on read.
func toAPIJSON(raw json.RawMessage) *interface{} {
	if len(raw) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return &v
}

func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// ListActionTypes serves the action-type catalog (D-ActionTypes, R-29) — a static registry read, so
// only an authenticated subject is required (no audit.read gate). Service codes are rendered as their
// module names (the pkg/rid vocabulary).
func (s Service) ListActionTypes(ctx context.Context, _ bearertoken.Token) ([]auditapi.ActionType, error) {
	names := rid.Services()
	cat := action.All()
	out := make([]auditapi.ActionType, 0, len(cat))
	for _, a := range cat {
		out = append(out, auditapi.ActionType{
			Code:       a.Code,
			Service:    names[a.Service],
			TargetType: a.TargetType,
			Permission: a.Permission,
			// Parameter schema single-sourced from the Conjure request type (R-29 seam); empty for
			// actions with no request body or not yet annotated (expand-only).
			Parameters: toActionParams(action.Params(a.Code)),
			// HTTP endpoint binding single-sourced from the IR (D-ActionInvocation, R-33); absent for
			// actions with no invocable endpoint (purge-cascade erasures, the import.* ingestion plane).
			Endpoint: toActionEndpoint(a.Code),
		})
	}
	return out, nil
}

func toActionEndpoint(code string) *auditapi.ActionEndpoint {
	e, ok := action.EndpointFor(code)
	if !ok {
		return nil
	}
	pp := e.PathParams
	if pp == nil {
		pp = []string{}
	}
	return &auditapi.ActionEndpoint{Method: e.Method, Path: e.Path, PathParams: pp}
}

func toActionParams(ps []action.Param) []auditapi.ActionParam {
	out := make([]auditapi.ActionParam, 0, len(ps))
	for _, p := range ps {
		ap := auditapi.ActionParam{Name: p.Name, Type: p.Type, Required: p.Required}
		if p.Docs != "" {
			d := p.Docs
			ap.Docs = &d
		}
		if p.Sensitivity != "" {
			s := p.Sensitivity
			ap.Sensitivity = &s
		}
		if len(p.Fields) > 0 {
			f := toActionParams(p.Fields) // one level deep; nested entries are flat (D-ActionInvocation R-33)
			ap.Fields = &f
		}
		out = append(out, ap)
	}
	return out
}
