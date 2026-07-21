package application

import (
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
)

// TestRejectDisabledCode pins the D-DataPacks (M54) rule that a disabled vertical's permission codes are
// not grantable: rejectDisabledCode returns ErrUnknownPermission for a code under a disabled prefix,
// and nil for everything else — including a code that merely SHARES a leading substring but not the
// dotted prefix boundary. Empty prefixes (no module disabled) reject nothing.
func TestRejectDisabledCode(t *testing.T) {
	s := &Service{disabledPrefixes: []string{"finance.", "religion.", "religionorg."}}

	for _, code := range []domain.Permission{"finance.read", "finance.catalog.manage", "religion.read", "religionorg.manage"} {
		if err := s.rejectDisabledCode(code); err == nil {
			t.Fatalf("%q under a disabled module should be rejected", code)
		}
	}
	for _, code := range []domain.Permission{"person.read", "vehicle.read", "financeXread", "import.manage"} {
		if err := s.rejectDisabledCode(code); err != nil {
			t.Fatalf("%q is not under a disabled module; must be allowed, got %v", code, err)
		}
	}

	// No module disabled → nothing rejected.
	none := &Service{}
	if err := none.rejectDisabledCode("finance.read"); err != nil {
		t.Fatalf("with no disabled prefixes, finance.read must be allowed, got %v", err)
	}
}
