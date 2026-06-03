// Package transport implements the platform module's generated Conjure server interface
// (docs/architecture/overview.md: transport implements the contract; D-Conjure). Generated code in
// internal/conjure is never hand-edited.
package transport

import (
	"context"

	platformapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/platform"
)

// OpsService implements platformapi.PlatformOpsService — the unauthenticated operational surface.
type OpsService struct {
	binaryRevision string
	schemaRevision string
}

// NewOpsService builds the ops service with the revisions it reports.
func NewOpsService(binaryRevision, schemaRevision string) OpsService {
	return OpsService{binaryRevision: binaryRevision, schemaRevision: schemaRevision}
}

// Version reports the binary + applied schema revision.
func (s OpsService) Version(ctx context.Context) (platformapi.VersionInfo, error) {
	return platformapi.VersionInfo{
		BinaryRevision: s.binaryRevision,
		SchemaRevision: s.schemaRevision,
	}, nil
}

// DemoError always returns Platform:DemoError, proving the werror -> Conjure SerializableError
// path end to end (M0 exit criterion). Not a real domain error.
func (s OpsService) DemoError(ctx context.Context) error {
	return platformapi.NewDemoError("demonstration of the Conjure SerializableError path")
}
