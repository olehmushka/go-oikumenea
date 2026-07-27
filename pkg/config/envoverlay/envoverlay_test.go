// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package envoverlay_test

import (
	"reflect"
	"strings"
	"testing"

	hconfig "github.com/olegamysk/go-oikumenea/internal/hermenea/config"
	config "github.com/olegamysk/go-oikumenea/internal/platform/config"
	"github.com/olegamysk/go-oikumenea/pkg/config/envoverlay"
	"gopkg.in/yaml.v3"
)

var installType = reflect.TypeOf(config.Install{})
var runtimeType = reflect.TypeOf(config.Runtime{})

// apply is a helper that overlays env (a literal map) onto base and unmarshals into a fresh
// config.Install, failing the test on any error.
func applyInstall(t *testing.T, base string, env map[string]string) config.Install {
	t.Helper()
	out, err := envoverlay.Apply([]byte(base), installType, "OIKUMENEA", env)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var got config.Install
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal overlaid yaml: %v\n---\n%s", err, out)
	}
	return got
}

func TestBindings_DashedDisambiguation(t *testing.T) {
	b := envoverlay.Bindings(installType, "OIKUMENEA")
	want := map[string][]string{
		"OIKUMENEA_CRYPTO_LOCAL_DEV_KEK":               {"crypto", "local-dev", "kek"},
		"OIKUMENEA_LOGIN_SECURITY_TRUST_FORWARDED_FOR": {"login-security", "trust-forwarded-for"},
		"OIKUMENEA_POSTGRES_DSN":                       {"postgres", "dsn"},
		"OIKUMENEA_ENVIRONMENT":                        {"environment"},
		"OIKUMENEA_IDP_CLOCK_SKEW_SECONDS":             {"idp", "clock-skew-seconds"},
		"OIKUMENEA_ACCOUNT_IDENTITY_LINKING_ENABLED":   {"account", "identity-linking-enabled"},
		"OIKUMENEA_BOOTSTRAP_ADMIN_PERSON_CODE":        {"bootstrap-admin", "person-code"},
		"OIKUMENEA_CRYPTO_DEK_CACHE_TTL_SECONDS":       {"crypto", "dek-cache-ttl-seconds"},
	}
	for env, path := range want {
		got, ok := b[env]
		if !ok {
			t.Errorf("missing binding %s", env)
			continue
		}
		if !reflect.DeepEqual([]string(got), path) {
			t.Errorf("%s: path = %v, want %v", env, got, path)
		}
	}
}

func TestBindings_InlineEmbedAndSlices(t *testing.T) {
	b := envoverlay.Bindings(installType, "OIKUMENEA")
	// Inline wconfig.Install embed folds in without a phantom segment.
	if _, ok := b["OIKUMENEA_SERVER_PORT"]; !ok {
		t.Error("expected OIKUMENEA_SERVER_PORT from inline wconfig.Install embed")
	}
	if _, ok := b["OIKUMENEA_PRODUCT_NAME"]; !ok {
		t.Error("expected OIKUMENEA_PRODUCT_NAME from inline embed")
	}
	// Struct-slice element placeholder name.
	if _, ok := b["OIKUMENEA_IDP_ISSUERS_N_HMAC_KEY"]; !ok {
		t.Errorf("expected OIKUMENEA_IDP_ISSUERS_N_HMAC_KEY, got keys: %v", keys(b))
	}
}

func TestBindings_HermeneaSources(t *testing.T) {
	b := envoverlay.Bindings(reflect.TypeOf(hconfig.Install{}), "HERMENEA")
	if _, ok := b["HERMENEA_SOURCES_N_LOCATOR"]; !ok {
		t.Errorf("expected HERMENEA_SOURCES_N_LOCATOR, got: %v", keys(b))
	}
}

func TestApply_ScalarPrecedenceAndFileless(t *testing.T) {
	base := "environment: local\npostgres:\n  dsn: from-yaml\n"
	// env overrides a present yaml scalar
	got := applyInstall(t, base, map[string]string{"OIKUMENEA_POSTGRES_DSN": "from-env"})
	if got.Postgres.DSN != "from-env" {
		t.Errorf("dsn = %q, want from-env", got.Postgres.DSN)
	}
	if got.Environment != "local" {
		t.Errorf("environment = %q, want local (untouched)", got.Environment)
	}
	// file-less: nil base + env only
	out, err := envoverlay.Apply(nil, installType, "OIKUMENEA", map[string]string{
		"OIKUMENEA_ENVIRONMENT":  "prod",
		"OIKUMENEA_POSTGRES_DSN": "env-dsn",
	})
	if err != nil {
		t.Fatalf("file-less Apply: %v", err)
	}
	var fl config.Install
	if err := yaml.Unmarshal(out, &fl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fl.Environment != "prod" || fl.Postgres.DSN != "env-dsn" {
		t.Errorf("file-less: %+v", fl)
	}
}

func TestApply_TypePreservation(t *testing.T) {
	got := applyInstall(t, "", map[string]string{
		"OIKUMENEA_LOGIN_SECURITY_RETENTION_DAYS":    "30",
		"OIKUMENEA_ACCOUNT_IDENTITY_LINKING_ENABLED": "false",
		"OIKUMENEA_BOOTSTRAP_ADMIN_PERSON_CODE":      "00700", // numeric-looking string stays a string
	})
	if got.LoginSecurity.RetentionDays != 30 {
		t.Errorf("retention-days = %d, want 30", got.LoginSecurity.RetentionDays)
	}
	if got.Account.IdentityLinkingEnabled == nil || *got.Account.IdentityLinkingEnabled != false {
		t.Errorf("identity-linking-enabled = %v, want false", got.Account.IdentityLinkingEnabled)
	}
	if got.BootstrapAdmin == nil || got.BootstrapAdmin.PersonCode != "00700" {
		t.Errorf("person-code = %+v, want string 00700", got.BootstrapAdmin)
	}
}

func TestApply_BadCoercionNamesVar(t *testing.T) {
	_, err := envoverlay.Apply(nil, installType, "OIKUMENEA", map[string]string{
		"OIKUMENEA_LOGIN_SECURITY_RETENTION_DAYS": "abc",
	})
	if err == nil || !strings.Contains(err.Error(), "OIKUMENEA_LOGIN_SECURITY_RETENTION_DAYS") {
		t.Errorf("expected error naming the var, got %v", err)
	}
}

func TestApply_StructSliceMergeAndExtend(t *testing.T) {
	base := "idp:\n  issuers:\n    - issuer: https://idp.example\n      audience: oik\n      type: oidc\n"
	got := applyInstall(t, base, map[string]string{
		"OIKUMENEA_IDP_ISSUERS_0_HMAC_KEY": "secret0",              // override only element 0's hmac-key
		"OIKUMENEA_IDP_ISSUERS_1_ISSUER":   "https://idp2.example", // extend a new element 1
	})
	if len(got.IDP.Issuers) != 2 {
		t.Fatalf("issuers = %d, want 2: %+v", len(got.IDP.Issuers), got.IDP.Issuers)
	}
	e0 := got.IDP.Issuers[0]
	if e0.Issuer != "https://idp.example" || e0.Audience != "oik" || e0.HMACKey != "secret0" {
		t.Errorf("element 0 merge wrong: %+v", e0)
	}
	if got.IDP.Issuers[1].Issuer != "https://idp2.example" {
		t.Errorf("element 1 = %+v", got.IDP.Issuers[1])
	}
}

func TestApply_ScalarSlice(t *testing.T) {
	got := applyInstall(t, "", map[string]string{
		"OIKUMENEA_CRYPTO_LOCAL_DEV_PREVIOUS_KEKS_0": "kek-a",
		"OIKUMENEA_CRYPTO_LOCAL_DEV_PREVIOUS_KEKS_1": "kek-b",
	})
	want := []string{"kek-a", "kek-b"}
	if !reflect.DeepEqual(got.Crypto.LocalDev.PreviousKEKs, want) {
		t.Errorf("previous-keks = %v, want %v", got.Crypto.LocalDev.PreviousKEKs, want)
	}
}

func TestApply_ModulesMap(t *testing.T) {
	got := applyInstall(t, "", map[string]string{"OIKUMENEA_MODULES_FINANCE_ENABLED": "false"})
	if got.ModuleEnabled("finance") {
		t.Errorf("finance should be disabled via env")
	}
	if !got.ModuleEnabled("religion") {
		t.Errorf("religion should default enabled")
	}
}

func TestApply_TopLevelNonMapping(t *testing.T) {
	_, err := envoverlay.Apply([]byte("- a\n- b\n"), installType, "OIKUMENEA", nil)
	if err == nil {
		t.Error("expected error for a top-level sequence")
	}
}

func TestApply_DBParts(t *testing.T) {
	// full DSN wins over parts
	got := applyInstall(t, "", map[string]string{
		"OIKUMENEA_POSTGRES_DSN": "explicit",
		"OIKUMENEA_DB_HOST":      "ignored",
	})
	if got.Postgres.DSN != "explicit" {
		t.Errorf("dsn = %q, want explicit (full DSN wins)", got.Postgres.DSN)
	}
	// assembly with default port
	got = applyInstall(t, "", map[string]string{
		"OIKUMENEA_DB_HOST":     "db",
		"OIKUMENEA_DB_USER":     "u",
		"OIKUMENEA_DB_PASSWORD": "p@ss word",
		"OIKUMENEA_DB_NAME":     "oik",
	})
	dsn := got.Postgres.DSN
	for _, sub := range []string{"host=db", "port=5432", "user=u", "dbname=oik", "password='p@ss word'"} {
		if !strings.Contains(dsn, sub) {
			t.Errorf("dsn %q missing %q", dsn, sub)
		}
	}
}

func TestApply_Aliases(t *testing.T) {
	out, err := envoverlay.ApplyWithAliases(nil, installType, "OIKUMENEA",
		map[string]string{"OIKUMENEA_HERMENEA_TOKEN": "tok"},
		map[string]envoverlay.Path{"OIKUMENEA_HERMENEA_TOKEN": {"hermenea", "outbound-token"}})
	if err != nil {
		t.Fatalf("ApplyWithAliases: %v", err)
	}
	var got config.Install
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Hermenea.OutboundToken != "tok" {
		t.Errorf("outbound-token = %q, want tok", got.Hermenea.OutboundToken)
	}
	if got.Hermenea.ResolveOutboundToken() != "tok" {
		t.Errorf("ResolveOutboundToken = %q, want tok", got.Hermenea.ResolveOutboundToken())
	}
}

func TestApply_RuntimeLoggingLevel(t *testing.T) {
	out, err := envoverlay.Apply(nil, runtimeType, "OIKUMENEA", map[string]string{
		"OIKUMENEA_LOGGING_LEVEL":     "debug",
		"OIKUMENEA_DEFAULT_PAGE_SIZE": "25",
	})
	if err != nil {
		t.Fatalf("Apply runtime: %v", err)
	}
	var rt config.Runtime
	if err := yaml.Unmarshal(out, &rt); err != nil {
		t.Fatalf("unmarshal runtime: %v\n%s", err, out)
	}
	if rt.DefaultPageSize != 25 {
		t.Errorf("default-page-size = %d, want 25", rt.DefaultPageSize)
	}
	if rt.LoggerConfig == nil || !strings.EqualFold(string(rt.LoggerConfig.Level), "debug") {
		t.Errorf("logging.level = %+v, want debug", rt.LoggerConfig)
	}
}

func keys(m map[string]envoverlay.Path) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
