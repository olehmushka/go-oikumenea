package envoverlay

import (
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// applyDBParts assembles postgres.dsn from discrete <PREFIX>_DB_* env vars as a libpq keyword string
// (pgx accepts it and it needs no URL-encoding of passwords). It is skipped entirely when the full
// <PREFIX>_POSTGRES_DSN is set (that wins). Because a DSN is one opaque string it is BUILT FROM PARTS
// ONLY — the DB_* parts do not field-merge into a yaml-supplied dsn.
func applyDBParts(root *yaml.Node, prefix string, env map[string]string) error {
	if _, ok := env[prefix+"_POSTGRES_DSN"]; ok {
		return nil
	}
	host, hOK := env[prefix+"_DB_HOST"]
	port, pOK := env[prefix+"_DB_PORT"]
	user, uOK := env[prefix+"_DB_USER"]
	pass, passOK := env[prefix+"_DB_PASSWORD"]
	name, nOK := env[prefix+"_DB_NAME"]
	ssl, sOK := env[prefix+"_DB_SSLMODE"]
	if !(hOK || pOK || uOK || passOK || nOK || sOK) {
		return nil
	}

	var parts []string
	if hOK {
		parts = append(parts, "host="+libpqQuote(host))
	}
	switch {
	case pOK:
		parts = append(parts, "port="+libpqQuote(port))
	case hOK:
		parts = append(parts, "port=5432") // sensible default when a host is given but no port
	}
	if uOK {
		parts = append(parts, "user="+libpqQuote(user))
	}
	if passOK {
		parts = append(parts, "password="+libpqQuote(pass))
	}
	if nOK {
		parts = append(parts, "dbname="+libpqQuote(name))
	}
	if sOK {
		parts = append(parts, "sslmode="+libpqQuote(ssl))
	}
	return setLeaf(root, Path{"postgres", "dsn"}, strings.Join(parts, " "), reflect.String)
}

// libpqQuote quotes a libpq connection-string value that contains whitespace or a quote/backslash,
// wrapping it in single quotes and escaping backslash and single-quote per libpq rules.
func libpqQuote(v string) string {
	if v == "" {
		return "''"
	}
	if !strings.ContainsAny(v, " \t'\\") {
		return v
	}
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + r.Replace(v) + "'"
}
