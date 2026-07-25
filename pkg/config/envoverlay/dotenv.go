package envoverlay

import (
	"bufio"
	"errors"
	"os"
	"reflect"
	"strings"
)

// dotEnvPath is the local convenience file loaded at boot. It is gitignored (.gitignore) — secrets
// there stay local.
const dotEnvPath = ".env"

// LoadDotEnv loads ./.env (if present) into the process environment, setting each key ONLY when it is
// not already set — so a real environment variable always wins over the .env value. A missing file is
// a no-op. Call once, early, per binary (before any config or token is read). Hand-rolled to avoid a
// third-party dependency for a trivial parser.
func LoadDotEnv() error {
	f, err := os.Open(dotEnvPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		i := strings.IndexByte(line, '=')
		if i < 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		if key == "" {
			continue
		}
		if _, ok := os.LookupEnv(key); ok {
			continue // real env wins over .env
		}
		if err := os.Setenv(key, unquoteDotEnv(strings.TrimSpace(line[i+1:]))); err != nil {
			return err
		}
	}
	return sc.Err()
}

// unquoteDotEnv strips a single matching pair of surrounding single or double quotes from a .env value.
func unquoteDotEnv(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

// LoadFileOverlay reads the config file at path (a missing file yields an empty base — env-only boot)
// and overlays the process environment for schema/prefix. It is the install-config entry point for the
// witchcraft ConfigBytesProvider and the CLI path.
func LoadFileOverlay(path string, schema reflect.Type, prefix string) ([]byte, error) {
	return LoadFileOverlayWithAliases(path, schema, prefix, nil)
}

// LoadFileOverlayWithAliases is LoadFileOverlay plus legacy env-name aliases (see ApplyWithAliases).
func LoadFileOverlayWithAliases(path string, schema reflect.Type, prefix string, aliases map[string]Path) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		raw = nil // file-less boot
	}
	return ApplyWithAliases(raw, schema, prefix, OSEnviron(), aliases)
}
