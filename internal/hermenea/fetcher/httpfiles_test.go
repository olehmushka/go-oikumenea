// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// TestHTTPFilesStage downloads a whitespace-separated URL list to a staged temp directory, keyed by
// basename, with a deterministic content-checksum version and a working cleanup.
func TestHTTPFilesStage(t *testing.T) {
	bodies := map[string]string{"/languages.csv": "ID,Name\nx,Y\n", "/values.csv": "Language_ID\nx\n"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b, ok := bodies[r.URL.Path]; ok {
			_, _ = w.Write([]byte(b))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	hf := HTTPFiles{client: srv.Client()}
	src := domain.Source{Locator: srv.URL + "/languages.csv  " + srv.URL + "/values.csv"}

	staged, err := hf.Stage(context.Background(), src)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	t.Cleanup(staged.Cleanup)

	for name, want := range map[string]string{"languages.csv": bodies["/languages.csv"], "values.csv": bodies["/values.csv"]} {
		b, err := os.ReadFile(filepath.Join(staged.Path, name))
		if err != nil {
			t.Fatalf("read staged %s: %v", name, err)
		}
		if string(b) != want {
			t.Fatalf("staged %s = %q, want %q", name, b, want)
		}
	}
	if staged.SourceVersion == "" || staged.Checksum == "" {
		t.Fatalf("missing version/checksum: %+v", staged)
	}

	// Deterministic: re-staging the same content yields the same version.
	again, err := hf.Stage(context.Background(), src)
	if err != nil {
		t.Fatalf("re-Stage: %v", err)
	}
	t.Cleanup(again.Cleanup)
	if again.SourceVersion != staged.SourceVersion {
		t.Fatalf("version not deterministic: %s vs %s", again.SourceVersion, staged.SourceVersion)
	}

	// Cleanup removes the staged dir.
	staged.Cleanup()
	if _, err := os.Stat(staged.Path); !os.IsNotExist(err) {
		t.Fatalf("staged dir not removed: %v", err)
	}
}

// TestHTTPFilesEmptyLocator rejects a source with no URLs.
func TestHTTPFilesEmptyLocator(t *testing.T) {
	if _, err := (HTTPFiles{client: http.DefaultClient}).Stage(context.Background(), domain.Source{Locator: "   "}); err == nil {
		t.Fatal("expected error for empty locator")
	}
}
