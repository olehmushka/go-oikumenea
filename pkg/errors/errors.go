// Package errors holds the werror conventions shared across modules (docs/architecture/conventions.md):
// wrap with safe/unsafe params, where safe params may appear in logs/responses and unsafe params
// (PII, secrets) are logged redacted and never returned. Module-specific Conjure error types are
// generated from each *.conjure.yml; this package carries the cross-cutting helpers.
package errors

import (
	"context"

	werror "github.com/palantir/witchcraft-go-error"
)

// Safe wraps err with a message and safe parameters (visible in logs and error responses).
func Safe(ctx context.Context, err error, msg string, safeParams map[string]any) error {
	return werror.WrapWithContextParams(ctx, err, msg, werror.SafeParams(safeParams))
}

// Unsafe wraps err with a message and unsafe parameters (PII/secrets — logged redacted, never
// returned to callers).
func Unsafe(ctx context.Context, err error, msg string, unsafeParams map[string]any) error {
	return werror.WrapWithContextParams(ctx, err, msg, werror.UnsafeParams(unsafeParams))
}
