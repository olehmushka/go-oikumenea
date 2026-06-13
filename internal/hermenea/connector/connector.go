// Package connector implements the hermenea Connector seam (M16 / D-Hermenea): pluggable fetchers
// that pull a source's raw payload. Two ship now — HTTP(S) download and the degenerate `file` case
// (bundled presets / deterministic tests). New source types (DS-44) are new Connector impls.
package connector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// maxPayload bounds a fetched body (16 MiB) so a runaway source can't exhaust memory.
const maxPayload = 16 << 20

// Registry maps a connector-type to its Connector.
type Registry map[string]domain.Connector

// Default builds the standard connector registry (http + file).
func Default() Registry {
	return Registry{
		domain.ConnectorHTTP: HTTP{client: &http.Client{Timeout: 30 * time.Second}},
		domain.ConnectorFile: File{},
	}
}

// HTTP fetches a source's payload over HTTP(S). source_version comes from a strong validator header
// (ETag / Last-Modified) when present, else a short content checksum.
type HTTP struct{ client *http.Client }

func (h HTTP) Fetch(ctx context.Context, src domain.Source) (domain.RawBatch, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.Locator, nil)
	if err != nil {
		return domain.RawBatch{}, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return domain.RawBatch{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.RawBatch{}, fmt.Errorf("connector http: %s returned %d", src.Locator, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPayload))
	if err != nil {
		return domain.RawBatch{}, err
	}
	sum := checksum(body)
	version := firstNonEmpty(resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"), sum[:12])
	return domain.RawBatch{Payload: body, SourceVersion: version, Checksum: sum}, nil
}

// File reads a source's payload from a bundled file (the degenerate connector; used for presets and
// deterministic tests). source_version is the file mod-time, falling back to a short checksum.
type File struct{}

func (File) Fetch(_ context.Context, src domain.Source) (domain.RawBatch, error) {
	body, err := os.ReadFile(src.Locator)
	if err != nil {
		return domain.RawBatch{}, err
	}
	sum := checksum(body)
	version := sum[:12]
	if fi, err := os.Stat(src.Locator); err == nil {
		version = fi.ModTime().UTC().Format(time.RFC3339)
	}
	return domain.RawBatch{Payload: body, SourceVersion: version, Checksum: sum}, nil
}

func checksum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
