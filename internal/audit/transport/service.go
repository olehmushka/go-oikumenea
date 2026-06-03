// Package transport implements the audit module's generated Conjure AuditService interface: it
// translates the wire contract to/from the application service and maps domain errors to Conjure
// SerializableErrors (overview.md; D-Conjure). Generated code in internal/conjure is never
// hand-edited.
//
// Authorization is intentionally not yet enforced here: the endpoints declare `auth: header` so
// the bearer token is parsed, but the `audit.read` permission check, unit-scoping over the
// closure, and the shadow gate are wired once authorization (M7) + identity-federation (M8) land
// (docs/modules/audit.md). The handlers receive the token and ignore it for now.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/audit/domain"
	auditapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/audit"
	cerrors "github.com/palantir/conjure-go-runtime/v2/conjure-go-contract/errors"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// Service adapts *application.Service to the generated auditapi.AuditService interface.
type Service struct {
	app *application.Service
}

// NewService builds the transport adapter over the audit application service.
func NewService(app *application.Service) Service {
	return Service{app: app}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ auditapi.AuditService = Service{}

// Query implements the GET /audit endpoint.
func (s Service) Query(
	ctx context.Context,
	_ bearertoken.Token,
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
func (s Service) Get(ctx context.Context, _ bearertoken.Token, entryID string) (auditapi.AuditEntry, error) {
	e, err := s.app.Get(ctx, entryID)
	if err != nil {
		return auditapi.AuditEntry{}, mapError(ctx, err, entryID)
	}
	return toAPIEntry(e), nil
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
