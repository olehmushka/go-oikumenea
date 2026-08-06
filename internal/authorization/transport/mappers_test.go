// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	authzapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/authorization"
	conjureerrors "github.com/palantir/conjure-go-runtime/v2/conjure-go-contract/errors"
)

// mapErrorContract pins the HTTP classification of every authorization domain error — the same guard
// the person, rank and identity-federation transports carry. The authorization transport had no such
// guard, which is how the M51 drift below went unnoticed.
//
// It catches two bug classes: (1) a new sentinel returned by the application but missing from the
// switch, so a client-input error falls through to `default:` and 500s (the M36 unknown-legalBasis
// bug); and (2) a sentinel classified as the WRONG typed error — invisible in a 4xx-vs-5xx check,
// because both mappings are 400s.
var mapErrorContract = []struct {
	name string
	err  error
	want func(error) bool
}{
	{"ErrRoleNotFound", domain.ErrRoleNotFound, authzapi.IsRoleNotFound},
	{"ErrRoleCodeConflict", domain.ErrRoleCodeConflict, authzapi.IsRoleConflict},
	{"ErrRoleIsBase", domain.ErrRoleIsBase, authzapi.IsRoleImmutable},
	{"ErrRoleInUse", domain.ErrRoleInUse, authzapi.IsRoleInUse},
	{"ErrRoleInvalid", domain.ErrRoleInvalid, authzapi.IsRoleInvalid},
	{"ErrAssignmentNotFound", domain.ErrAssignmentNotFound, authzapi.IsAssignmentNotFound},
	{"ErrAssignmentConflict", domain.ErrAssignmentConflict, authzapi.IsAssignmentConflict},
	{"ErrAssignmentInvalid", domain.ErrAssignmentInvalid, authzapi.IsAssignmentInvalid},
	{"ErrNonAuthorityBearingGraph", domain.ErrNonAuthorityBearingGraph, authzapi.IsNonAuthorityBearingGraph},
	{"ErrSelfEscalation", domain.ErrSelfEscalation, authzapi.IsSelfEscalation},
	{"ErrUnknownSubject", domain.ErrUnknownSubject, authzapi.IsAssignmentInvalid},
	{"ErrUnknownRole", domain.ErrUnknownRole, authzapi.IsAssignmentInvalid},
	{"ErrUnknownUnit", domain.ErrUnknownUnit, authzapi.IsAssignmentInvalid},
	{"ErrUnknownGraph", domain.ErrUnknownGraph, authzapi.IsAssignmentInvalid},
	{"ErrInstanceAdminNotFound", domain.ErrInstanceAdminNotFound, authzapi.IsInstanceAdminNotFound},
	{"ErrInstanceAdminConflict", domain.ErrInstanceAdminConflict, authzapi.IsInstanceAdminConflict},
	{"ErrPermissionDenied", domain.ErrPermissionDenied, authzapi.IsPermissionDenied},
	// M51 service principals. mapError is the ROLE-path mapper, so a shared sentinel is classified
	// here as the role endpoints need it; mapPrincipalError overrides where the two diverge (below).
	{"ErrPrincipalGrantNotFound", domain.ErrPrincipalGrantNotFound, authzapi.IsPrincipalGrantNotFound},
	{"ErrPrincipalGrantConflict", domain.ErrPrincipalGrantConflict, authzapi.IsPrincipalGrantConflict},
	{"ErrPrincipalGrantInvalid", domain.ErrPrincipalGrantInvalid, authzapi.IsPrincipalGrantInvalid},
	{"ErrUnknownPrincipal", domain.ErrUnknownPrincipal, authzapi.IsPrincipalGrantInvalid},
	{"ErrUnknownOrganization", domain.ErrUnknownOrganization, authzapi.IsPrincipalGrantInvalid},
	// Raised by BOTH paths: createRole/updateRole with a code outside the closed catalog, and
	// grantPrincipalPermission likewise. The shared mapper serves the role endpoints.
	{"ErrUnknownPermission", domain.ErrUnknownPermission, authzapi.IsRoleInvalid},
}

// mapPrincipalErrorContract pins the principal-grant endpoints' mapper where it DIVERGES from the
// shared one. Only context-sensitive sentinels belong here; everything else must fall through to
// mapError unchanged, which TestPrincipalMapperDelegates proves.
//
// This is the M52 regression test for real drift: granting an unknown permission code returned
// `Role:RoleInvalid`, but api/authorization.conjure.yml declares
// `PrincipalGrant:PrincipalGrantInvalid` for that endpoint. Both are 400s, so only an assertion on
// the specific typed error catches it.
var mapPrincipalErrorContract = []struct {
	name string
	err  error
	want func(error) bool
}{
	{"ErrUnknownPermission", domain.ErrUnknownPermission, authzapi.IsPrincipalGrantInvalid},
}

// TestMapErrorClassifiesEverySentinel proves each contracted domain error maps to its typed Conjure
// error and NOT the `default:` 500 wrap.
func TestMapErrorClassifiesEverySentinel(t *testing.T) {
	ctx := context.Background()
	var svc Service // mapError is a value receiver and reads no Service fields
	for _, tc := range mapErrorContract {
		got := svc.mapError(ctx, tc.err)
		if !tc.want(got) {
			t.Errorf("mapError(%s) = %T (%v); want the contracted typed error, not this", tc.name, got, got)
		}
	}
}

// TestMapPrincipalErrorOverrides proves the principal-grant mapper reclassifies the sentinels the
// contract asks it to.
func TestMapPrincipalErrorOverrides(t *testing.T) {
	ctx := context.Background()
	var svc Service
	for _, tc := range mapPrincipalErrorContract {
		got := svc.mapPrincipalError(ctx, tc.err)
		if !tc.want(got) {
			t.Errorf("mapPrincipalError(%s) = %T (%v); want the principal-grant typed error", tc.name, got, got)
		}
	}
}

// TestPrincipalMapperDelegates proves mapPrincipalError is a thin override: every sentinel it does NOT
// deliberately reclassify must map exactly as mapError does. Without this, the two mappers could drift
// apart silently as new sentinels are added to the shared switch.
func TestPrincipalMapperDelegates(t *testing.T) {
	ctx := context.Background()
	var svc Service
	overridden := make(map[string]bool, len(mapPrincipalErrorContract))
	for _, tc := range mapPrincipalErrorContract {
		overridden[tc.name] = true
	}
	for _, tc := range mapErrorContract {
		if overridden[tc.name] {
			continue
		}
		// Compare the Conjure error NAME + CODE, not the rendered string: every constructor mints a
		// fresh errorInstanceId, so two identical mappings never stringify equal.
		sharedName, sharedCode := conjureIdentity(svc.mapError(ctx, tc.err))
		principalName, principalCode := conjureIdentity(svc.mapPrincipalError(ctx, tc.err))
		if sharedName != principalName || sharedCode != principalCode {
			t.Errorf("mapPrincipalError(%s) = %s/%s; diverges from mapError = %s/%s without being declared in mapPrincipalErrorContract",
				tc.name, principalCode, principalName, sharedCode, sharedName)
		}
	}
}

// conjureIdentity extracts the (name, code) pair identifying a typed Conjure error. A non-Conjure
// error (the `default:` 500 wrap) yields empty strings, so an unmapped sentinel still compares equal
// to an unmapped sentinel — that case is TestMapErrorClassifiesEverySentinel's job, not this one.
func conjureIdentity(err error) (name, code string) {
	var ce conjureerrors.Error
	if errors.As(err, &ce) {
		return ce.Name(), ce.Code().String()
	}
	return "", ""
}

// TestMapErrorContractCoversEverySentinel is the drift guard: every exported Err* sentinel declared in
// the authorization domain package must appear in mapErrorContract above (and is therefore proven to
// map to a typed non-500 error). Adding a sentinel without classifying it fails here.
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
