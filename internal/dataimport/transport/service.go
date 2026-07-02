// Package transport implements the generated Conjure ImportService interface (M16 / D-Hermenea): it
// gates on the import service principal / import.manage (PEP), validates the envelope against the path
// object-type, maps the wire envelope onto the application type, and maps domain errors to Conjure
// SerializableErrors. Generated code in internal/conjure is never hand-edited.
package transport

import (
	"context"
	"errors"

	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	dataimportapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/dataimport"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/application"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	conjureerrors "github.com/palantir/conjure-go-runtime/v2/conjure-go-contract/errors"
	"github.com/palantir/pkg/bearertoken"
)

// Service adapts *application.Service to the generated dataimportapi.ImportService interface.
type Service struct {
	app *application.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the import application service and the PEP enforcer.
func NewService(app *application.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, pep: enforcer}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ dataimportapi.ImportService = Service{}

// ImportObjects upserts a canonical envelope into the {objectType} catalog (idempotent,
// non-destructive). It enforces the import permission, checks the envelope's objectType matches the
// path, decodes the untyped records into JSON objects, and runs the registered handler.
func (s Service) ImportObjects(ctx context.Context, token bearertoken.Token, objectType string, env dataimportapi.CanonicalEnvelope) (dataimportapi.ImportResult, error) {
	if err := s.pep.RequireImport(ctx, token); err != nil {
		return dataimportapi.ImportResult{}, err
	}
	if env.ObjectType != objectType {
		return dataimportapi.ImportResult{}, dataimportapi.NewEnvelopeMismatch(objectType, env.ObjectType)
	}
	records := make([]domain.Record, 0, len(env.Records))
	for _, r := range env.Records {
		m, ok := r.(map[string]any)
		if !ok {
			return dataimportapi.ImportResult{}, conjureerrors.NewInvalidArgument()
		}
		records = append(records, m)
	}
	appEnv := application.Envelope{
		ObjectType:    objectType,
		Source:        env.Source,
		SourceVersion: deref(env.SourceVersion),
		Records:       records,
	}
	sum, err := s.app.Import(ctx, objectType, appEnv)
	switch {
	case errors.Is(err, domain.ErrUnknownObjectType):
		return dataimportapi.ImportResult{}, dataimportapi.NewUnknownObjectType(objectType)
	case errors.Is(err, domain.ErrInvalidRecord):
		return dataimportapi.ImportResult{}, conjureerrors.NewInvalidArgument()
	case err != nil:
		return dataimportapi.ImportResult{}, err
	}
	return dataimportapi.ImportResult{
		ObjectType: objectType,
		Created:    sum.Created,
		Updated:    sum.Updated,
		Skipped:    sum.Skipped,
	}, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
