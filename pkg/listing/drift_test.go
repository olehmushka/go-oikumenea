// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package listing_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// These guards keep M56's consolidation from silently unwinding. They parse the AST rather than
// grepping, so a doc comment that merely NAMES the old shape (several of them do, explaining why it
// was replaced) does not trip them.

const internalRoot = "../../internal"

// Guard 1 — a page token rides in a query parameter, so it must be URL-safe base64. StdEncoding's
// `+` decodes to a space in a query string and silently corrupts the cursor; URLEncoding's `=`
// padding needs escaping. pkg/listing emits RawURL for everyone, so no module needs either alphabet.
// (pkg/listing itself still REFERENCES them, in its tolerant decoder — that is why this walks
// internal/ and not the whole repo.)
func TestNoNonURLSafeBase64InModules(t *testing.T) {
	forEachGoFile(t, func(t *testing.T, path string, file *ast.File) {
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "base64" {
				return true
			}
			if sel.Sel.Name == "StdEncoding" || sel.Sel.Name == "URLEncoding" {
				t.Errorf("%s uses base64.%s for a page token; page tokens must be URL-safe — "+
					"use pkg/listing (RawURL) instead", path, sel.Sel.Name)
			}
			return true
		})
	})
}

// Guard 2 — a file that declares a page-token codec must delegate to pkg/listing rather than
// re-implementing one. Thirteen modules had drifted into three incompatible copies before M56.
func TestPageTokenCodecsDelegateToListing(t *testing.T) {
	codecNames := map[string]bool{
		"encodeCursor": true, "decodeCursor": true,
		"encodeToken": true, "decodeToken": true,
		"encodeIDCursor": true, "decodeIDCursor": true,
		"encodeNearCursor": true, "decodeNearCursor": true,
	}
	forEachGoFile(t, func(t *testing.T, path string, file *ast.File) {
		var declared []string
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if ok && fn.Recv == nil && codecNames[fn.Name.Name] {
				declared = append(declared, fn.Name.Name)
			}
		}
		if len(declared) == 0 {
			return
		}
		if !imports(file, "github.com/olehmushka/go-oikumenea/pkg/listing") {
			t.Errorf("%s declares page-token codec(s) %v without importing pkg/listing — "+
				"the codec is shared (M56); do not re-implement it", path, declared)
		}
	})
}

// Guard 3 — same for the page-size clamp: declare the bounds, delegate the clamping.
func TestPageSizeClampsDelegateToListing(t *testing.T) {
	clampNames := map[string]bool{"clampPageSize": true, "pageSizeOr": true, "resolvePageSize": true}
	forEachGoFile(t, func(t *testing.T, path string, file *ast.File) {
		var declared []string
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if ok && clampNames[fn.Name.Name] {
				declared = append(declared, fn.Name.Name)
			}
		}
		if len(declared) == 0 {
			return
		}
		if !imports(file, "github.com/olehmushka/go-oikumenea/pkg/listing") {
			t.Errorf("%s declares page-size clamp(s) %v without importing pkg/listing — "+
				"use listing.PageSize with the module's own Default/Max", path, declared)
		}
	})
}

// The guards must actually be looking at something; a broken walk would pass vacuously.
func TestGuardsScanTheModules(t *testing.T) {
	n := 0
	forEachGoFile(t, func(*testing.T, string, *ast.File) { n++ })
	if n < 100 {
		t.Fatalf("walked only %d files under %s — the guards are not scanning the modules", n, internalRoot)
	}
}

func forEachGoFile(t *testing.T, fn func(*testing.T, string, *ast.File)) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.WalkDir(internalRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// internal/conjure is Conjure-generated and never hand-edited.
		if d.IsDir() && d.Name() == "conjure" {
			return fs.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return perr
		}
		fn(t, path, f)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", internalRoot, err)
	}
}

func imports(file *ast.File, path string) bool {
	for _, imp := range file.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == path {
			return true
		}
	}
	return false
}
