// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	identityapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/identityfederation"
	"github.com/olehmushka/go-oikumenea/internal/identityfederation/domain"
)

// mapErrorContract pins the HTTP classification of every identity-federation domain error mapError
// translates — the same guard the person and rank transports carry. It catches the bug class where a
// new sentinel is returned by the application but missing from the switch, so a client-input error
// falls through to the `default:` and 500s. That is exactly what happened in M36 (an unknown
// legalBasis 500'd), so the M51 principal sentinels are pinned here from the start.
var mapErrorContract = []struct {
	name string
	err  error
	want func(error) bool
}{
	{"ErrAccountNotFound", domain.ErrAccountNotFound, identityapi.IsAccountNotFound},
	{"ErrAccountConflict", domain.ErrAccountConflict, identityapi.IsAccountConflict},
	{"ErrAccountInvalid", domain.ErrAccountInvalid, identityapi.IsAccountInvalid},
	{"ErrUnknownPerson", domain.ErrUnknownPerson, identityapi.IsAccountInvalid},
	{"ErrIdentityNotFound", domain.ErrIdentityNotFound, identityapi.IsIdentityNotFound},
	{"ErrIdentityConflict", domain.ErrIdentityConflict, identityapi.IsIdentityConflict},
	{"ErrIdentityInvalid", domain.ErrIdentityInvalid, identityapi.IsIdentityInvalid},
	{"ErrLinkingDisabled", domain.ErrLinkingDisabled, identityapi.IsIdentityConflict},
	// M51 service principals.
	{"ErrPrincipalNotFound", domain.ErrPrincipalNotFound, identityapi.IsServicePrincipalNotFound},
	{"ErrPrincipalConflict", domain.ErrPrincipalConflict, identityapi.IsServicePrincipalConflict},
	{"ErrPrincipalInvalid", domain.ErrPrincipalInvalid, identityapi.IsServicePrincipalInvalid},
	// Re-pointing an immutable identity key is CLIENT input, not a server fault -> 400, not 500.
	{"ErrPrincipalIdentityImmutable", domain.ErrPrincipalIdentityImmutable, identityapi.IsServicePrincipalInvalid},
}

// TestMapErrorClassifiesEverySentinel proves each contracted domain error maps to its typed Conjure
// error and NOT the `default:` 500 wrap.
func TestMapErrorClassifiesEverySentinel(t *testing.T) {
	ctx := context.Background()
	var svc Service // mapError is a value receiver and reads no Service fields
	for _, tc := range mapErrorContract {
		got := svc.mapError(ctx, tc.err, errCtx{})
		if !tc.want(got) {
			t.Errorf("mapError(%s) = %T (%v); want a typed 4xx Conjure error, not the 500 default", tc.name, got, got)
		}
	}
}

// TestMapErrorContractCoversEverySentinel is the drift guard: every exported Err* sentinel declared in
// the identity-federation domain package must appear in mapErrorContract above (and is therefore
// proven to map to a typed non-500 error). Adding a sentinel without classifying it fails here.
func TestMapErrorContractCoversEverySentinel(t *testing.T) {
	covered := make(map[string]bool, len(mapErrorContract))
	for _, tc := range mapErrorContract {
		covered[tc.name] = true
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, "../domain", nil, 0)
	if err != nil {
		t.Fatalf("parse domain package: %v", err)
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				vs, ok := n.(*ast.ValueSpec)
				if !ok {
					return true
				}
				for _, name := range vs.Names {
					if !strings.HasPrefix(name.Name, "Err") || !name.IsExported() {
						continue
					}
					if !covered[name.Name] {
						t.Errorf("domain.%s is not in mapErrorContract — classify it, or it will 500 as an unmapped error", name.Name)
					}
				}
				return true
			})
		}
	}
}
