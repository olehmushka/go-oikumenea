// Package config holds framework-free helpers for reading hot-reloadable runtime configuration
// (docs/architecture/conventions.md: ECV + refreshable). It depends only on the generic
// refreshable library, never on witchcraft, so it stays usable from any layer.
package config

import (
	"github.com/palantir/pkg/refreshable"
)

// IntOrDefault derives an int Refreshable from base by applying extract, substituting fallback
// whenever extract yields a non-positive value. Modules use this to read tunables (e.g. the
// default page size) without importing the platform install/runtime config types.
func IntOrDefault(base refreshable.Refreshable, fallback int, extract func(any) int) refreshable.Refreshable {
	return base.Map(func(v any) any {
		if n := extract(v); n > 0 {
			return n
		}
		return fallback
	})
}
