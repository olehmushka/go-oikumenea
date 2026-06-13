package rid

import "testing"

func TestDecodeFields(t *testing.T) {
	// A crafted person/object/person RID: byte6=0x81 (v8|kind1), byte7=0x01 (app), byte8=0x86
	// (variant|service6), byte9=0x01 (type low=1), byte10 hi nibble 0 (type=1).
	r, err := Parse("0192f3a1-0000-8101-8601-0abcdef01234")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.App() != App {
		t.Fatalf("app = %d, want %d", r.App(), App)
	}
	if r.Service() != SvcPerson {
		t.Fatalf("service = %d, want %d", r.Service(), SvcPerson)
	}
	if r.Kind() != KindObject {
		t.Fatalf("kind = %v, want object", r.Kind())
	}
	if r.TypeCode() != 1 || r.TypeName() != "person" {
		t.Fatalf("type = %d/%q, want 1/person", r.TypeCode(), r.TypeName())
	}
	if r.Version() != 8 {
		t.Fatalf("version = %d, want 8", r.Version())
	}
}

func TestRenderParseRoundTrip(t *testing.T) {
	// authz/link/has_role: byte6=0x82 (kind2), byte8=0x88 (service8), byte9=0x01 (type1).
	const uuid = "0192f3a1-0000-8201-8801-000000000000"
	r := MustParse(uuid)
	want := "oikumenea:authz:link:has_role:" + uuid
	if got := r.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	// Parsing the rendered form must recover the same RID.
	back, err := Parse(r.String())
	if err != nil {
		t.Fatalf("parse rendered: %v", err)
	}
	if back.UUID() != r.UUID() {
		t.Fatalf("round-trip uuid = %q, want %q", back.UUID(), r.UUID())
	}
}

func TestTypeCodeHighBits(t *testing.T) {
	// type code 10 (person social_handle): low byte 0x0A, high nibble 0. And a 9-bit code 0x101=257
	// to exercise the high nibble: byte9=0x01, byte10 hi nibble=0x1.
	if r := MustParse("00000000-0000-8101-860a-000000000000"); r.TypeCode() != 10 || r.TypeName() != "social_handle" {
		t.Fatalf("type = %d/%q, want 10/social_handle", r.TypeCode(), r.TypeName())
	}
	if r := MustParse("00000000-0000-8101-8601-100000000000"); r.TypeCode() != 0x101 {
		t.Fatalf("high-bit type = %d, want %d", r.TypeCode(), 0x101)
	}
}

func TestHelpers(t *testing.T) {
	action := MustParse("00000000-0000-8300-8000-000000000000") // kind=3
	if !IsAction(action.UUID()) {
		t.Fatal("IsAction false for action RID")
	}
	if IsAction("00000000-0000-8100-8000-000000000000") {
		t.Fatal("IsAction true for object RID")
	}
	if !IsRID("00000000-0000-8101-8601-000000000000") {
		t.Fatal("IsRID false for valid RID")
	}
	if IsRID("not-a-uuid") {
		t.Fatal("IsRID true for junk")
	}
	// LinkType: person link type 2 -> partnered_with; wrong service -> "".
	if got := LinkType("00000000-0000-8201-8602-000000000000", SvcPerson); got != "partnered_with" {
		t.Fatalf("LinkType = %q, want partnered_with", got)
	}
	if got := LinkType("00000000-0000-8201-8602-000000000000", SvcAuthz); got != "" {
		t.Fatalf("LinkType wrong-service = %q, want empty", got)
	}
}
