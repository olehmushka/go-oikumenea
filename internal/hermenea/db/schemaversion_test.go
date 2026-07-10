package db

import "testing"

// TestExpectedSchemaRevisionDerivedFromEmbed proves the R-25 gate derives its expected revision from
// the embedded hermenea migrations (no hand-bumped constant): the marker in the latest migration is
// what ExpectedSchemaRevision resolves to. Adding a future hermenea migration that bumps the marker
// updates this automatically — this test then tracks the new latest revision.
func TestExpectedSchemaRevisionDerivedFromEmbed(t *testing.T) {
	if ExpectedSchemaRevision != "0006_schema_version" {
		t.Fatalf("ExpectedSchemaRevision = %q, want %q (derived from the embedded migrations/hermenea)",
			ExpectedSchemaRevision, "0006_schema_version")
	}
}
