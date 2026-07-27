// Package transport implements the generated connectorapi.ConnectorService (M53 / D-ConnectorPlane).
// Two audiences, two gate families:
//
//   - self-service (registerConnector / reportSyncRun) is for MACHINE subjects — gated with
//     RequireService on `connector.register` / `connector.report`, and the principal is taken from the
//     request context (never the wire), so a connector cannot act as another.
//   - the read surfaces are for OPERATORS — gated with RequireServiceOrPerson on `connector.read`, so
//     both an instance admin and a suitably-granted machine can read the fleet.
//
// Domain sentinels map to the Conjure Connector:* SerializableErrors. Generated code is never hand-edited.
package transport

import (
	"context"
	"errors"
	"time"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	connectorapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/connector"
	"github.com/olegamysk/go-oikumenea/internal/connector/application"
	"github.com/olegamysk/go-oikumenea/internal/connector/domain"
	"github.com/olegamysk/go-oikumenea/pkg/authn"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

const (
	registerPerm = string(authzdomain.PermConnectorRegister)
	reportPerm   = string(authzdomain.PermConnectorReport)
	readPerm     = string(authzdomain.PermConnectorRead)
)

// ConnectorService adapts *application.Service to the generated Conjure interface.
type ConnectorService struct {
	app *application.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the application service and the PEP enforcer.
func NewService(app *application.Service, enforcer *pep.Enforcer) ConnectorService {
	return ConnectorService{app: app, pep: enforcer}
}

var _ connectorapi.ConnectorService = ConnectorService{}

// ============================ self-service (machine subjects) ============================

func (s ConnectorService) RegisterConnector(ctx context.Context, token bearertoken.Token, req connectorapi.RegisterConnectorRequest) (connectorapi.Connector, error) {
	// Machine-only, no org dimension (M53) → orgID "". RequireService denies a non-machine subject.
	if err := s.pep.RequireService(ctx, token, registerPerm, ""); err != nil {
		return connectorapi.Connector{}, err
	}
	in := domain.RegistrationInput{
		PrincipalID: authn.PrincipalID(ctx), // from context — never from the request
		Code:        req.Code,
		Name:        req.Name,
		Description: derefStr(req.Description),
		Sources:     toSourceDecls(req.Sources),
	}
	c, _, err := s.app.Register(ctx, in)
	if err != nil {
		return connectorapi.Connector{}, s.mapError(err)
	}
	return toAPIConnector(c), nil
}

func (s ConnectorService) ReportSyncRun(ctx context.Context, token bearertoken.Token, req connectorapi.ReportSyncRunRequest) (connectorapi.SyncRun, error) {
	if err := s.pep.RequireService(ctx, token, reportPerm, ""); err != nil {
		return connectorapi.SyncRun{}, err
	}
	in := domain.ReportInput{
		PrincipalID:   authn.PrincipalID(ctx),
		SourceCode:    req.SourceCode,
		ExternalRunID: derefStr(req.ExternalRunId),
		State:         req.State,
		Created:       int64(derefInt(req.Created)),
		Updated:       int64(derefInt(req.Updated)),
		Skipped:       int64(derefInt(req.Skipped)),
		Error:         derefStr(req.Error),
		StartedAt:     toTimePtr(req.StartedAt),
		FinishedAt:    toTimePtr(req.FinishedAt),
	}
	run, err := s.app.Report(ctx, in)
	if err != nil {
		return connectorapi.SyncRun{}, s.mapError(err)
	}
	return toAPISyncRun(run), nil
}

// ============================ operator reads ============================

func (s ConnectorService) ListConnectors(ctx context.Context, token bearertoken.Token, pageSize *int, pageToken *string) (connectorapi.ConnectorPage, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, readPerm, ""); err != nil {
		return connectorapi.ConnectorPage{}, err
	}
	after, err := decodeToken(pageToken)
	if err != nil {
		return connectorapi.ConnectorPage{}, err
	}
	lim := derefInt(pageSize)
	rows, err := s.app.ListConnectors(ctx, after, lim)
	if err != nil {
		return connectorapi.ConnectorPage{}, s.mapError(err)
	}
	out := make([]connectorapi.Connector, 0, len(rows))
	for _, c := range rows {
		out = append(out, toAPIConnector(c))
	}
	return connectorapi.ConnectorPage{Connectors: out, NextPageToken: nextToken(rows, lim, func(i int) string { return rows[i].ID })}, nil
}

func (s ConnectorService) GetConnector(ctx context.Context, token bearertoken.Token, connectorID string) (connectorapi.Connector, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, readPerm, ""); err != nil {
		return connectorapi.Connector{}, err
	}
	c, err := s.app.GetConnector(ctx, connectorID)
	if err != nil {
		return connectorapi.Connector{}, s.mapError(err)
	}
	return toAPIConnector(c), nil
}

func (s ConnectorService) ListConnectorSources(ctx context.Context, token bearertoken.Token, connectorID string) (connectorapi.ConnectorSourceList, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, readPerm, ""); err != nil {
		return connectorapi.ConnectorSourceList{}, err
	}
	rows, err := s.app.ListSources(ctx, connectorID)
	if err != nil {
		return connectorapi.ConnectorSourceList{}, s.mapError(err)
	}
	out := make([]connectorapi.ConnectorSource, 0, len(rows))
	for _, src := range rows {
		out = append(out, toAPISource(src))
	}
	return connectorapi.ConnectorSourceList{Sources: out}, nil
}

func (s ConnectorService) ListSyncRuns(ctx context.Context, token bearertoken.Token, sourceID *string, pageSize *int, pageToken *string) (connectorapi.SyncRunPage, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, readPerm, ""); err != nil {
		return connectorapi.SyncRunPage{}, err
	}
	after, err := decodeToken(pageToken)
	if err != nil {
		return connectorapi.SyncRunPage{}, err
	}
	lim := derefInt(pageSize)
	rows, err := s.app.ListRuns(ctx, derefStr(sourceID), after, lim)
	if err != nil {
		return connectorapi.SyncRunPage{}, s.mapError(err)
	}
	out := make([]connectorapi.SyncRun, 0, len(rows))
	for _, run := range rows {
		out = append(out, toAPISyncRun(run))
	}
	return connectorapi.SyncRunPage{Runs: out, NextPageToken: nextToken(rows, lim, func(i int) string { return rows[i].ID })}, nil
}

// ============================ error mapping ============================

func (s ConnectorService) mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrConnectorNotFound):
		return connectorapi.NewConnectorNotFound("")
	case errors.Is(err, domain.ErrSourceNotFound):
		return connectorapi.NewSourceNotFound("")
	case errors.Is(err, domain.ErrConflict):
		return connectorapi.NewConnectorConflict("the connector code is registered to a different principal")
	case errors.Is(err, domain.ErrNoPrincipal):
		return connectorapi.NewConnectorInvalid("self-registration requires a service principal")
	case errors.Is(err, domain.ErrInvalid):
		return connectorapi.NewConnectorInvalid(err.Error())
	default:
		return werror.Wrap(err, "connector request failed")
	}
}

// ============================ wire mapping ============================

func toAPIConnector(c domain.Connector) connectorapi.Connector {
	out := connectorapi.Connector{
		Id:     c.ID,
		Code:   c.Code,
		Name:   c.Name,
		Status: c.Status,
	}
	out.Description = strPtr(c.Description)
	out.PrincipalId = strPtr(c.PrincipalID)
	if c.LastSeenAt != nil {
		dt := datetime.DateTime(*c.LastSeenAt)
		out.LastSeenAt = &dt
	}
	out.CreatedAt = datetime.DateTime(c.CreatedAt)
	out.UpdatedAt = datetime.DateTime(c.UpdatedAt)
	return out
}

func toAPISource(src domain.Source) connectorapi.ConnectorSource {
	return connectorapi.ConnectorSource{
		Id:          src.ID,
		ConnectorId: src.ConnectorID,
		Code:        src.Code,
		Name:        src.Name,
		ObjectType:  strPtr(src.ObjectType),
		Schedule:    strPtr(src.Schedule),
		Enabled:     src.Enabled,
	}
}

func toAPISyncRun(r domain.SyncRun) connectorapi.SyncRun {
	out := connectorapi.SyncRun{
		Id:            r.ID,
		SourceId:      r.SourceID,
		ExternalRunId: strPtr(r.ExternalRunID),
		State:         r.State,
		Created:       int(r.Created),
		Updated:       int(r.Updated),
		Skipped:       int(r.Skipped),
		StartedAt:     datetime.DateTime(r.StartedAt),
	}
	out.Error = strPtr(r.Error)
	if r.FinishedAt != nil {
		dt := datetime.DateTime(*r.FinishedAt)
		out.FinishedAt = &dt
	}
	return out
}

func toSourceDecls(in []connectorapi.SourceDeclaration) []domain.SourceDeclaration {
	out := make([]domain.SourceDeclaration, 0, len(in))
	for _, d := range in {
		out = append(out, domain.SourceDeclaration{
			Code:       d.Code,
			Name:       d.Name,
			ObjectType: derefStr(d.ObjectType),
			Schedule:   derefStr(d.Schedule),
			Enabled:    d.Enabled == nil || *d.Enabled, // absent = true (SourceDeclaration doc)
		})
	}
	return out
}

// ============================ small helpers ============================

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func toTimePtr(dt *datetime.DateTime) *time.Time {
	if dt == nil {
		return nil
	}
	t := time.Time(*dt)
	return &t
}

// strPtr returns nil for "" so an absent value serializes as omitted, not empty-string.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// decodeToken/encodeToken are opaque keyset cursors over the RID, delegated to the shared codec
// (M56). Unlike the other transports this one REJECTS an undecodable token rather than silently
// restarting at page 1 — the stricter behaviour, kept as-is.
func decodeToken(tok *string) (string, error) {
	id, err := listing.DecodeCursorPtr(tok)
	if err != nil {
		return "", connectorapi.NewConnectorInvalid("invalid page token")
	}
	return id, nil
}

func encodeToken(id string) *string {
	t := listing.EncodeCursor(id)
	return &t
}

// nextToken returns the cursor for the next page: the id of the last row when the page filled to the
// limit, else nil (last page). `lim<=0` means the default page was used, so it always fills — a nil
// token only appears once fewer than `lim` rows come back, which pageSize's default handles.
func nextToken[T any](rows []T, lim int, idAt func(int) string) *string {
	if lim <= 0 || len(rows) < lim {
		return nil
	}
	return encodeToken(idAt(len(rows) - 1))
}
