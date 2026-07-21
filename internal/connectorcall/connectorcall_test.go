package connectorcall

import (
	"testing"
	"time"
)

// TestDeadlineIsBounded pins the R-12 invariant: the on-demand-lookup deadline is a real, non-zero
// bound. A zero or absent deadline is exactly the coupling this seam exists to prevent.
func TestDeadlineIsBounded(t *testing.T) {
	if Deadline <= 0 {
		t.Fatalf("Deadline must be a positive bound, got %v", Deadline)
	}
	if Deadline > 30*time.Second {
		t.Fatalf("Deadline %v is too loose for a synchronous request-path lookup (R-12)", Deadline)
	}
}

// TestDialRejectsEmptyBaseURL: Dial refuses to build a client with no target rather than producing one
// that fails obscurely at call time.
func TestDialRejectsEmptyBaseURL(t *testing.T) {
	if _, err := Dial("", false); err == nil {
		t.Fatal("Dial(\"\") = nil error; want a rejection")
	}
}

// TestDialBuildsClient: a real base URL yields a usable client (the deadline is applied inside, not by
// the caller — the whole point of the seam).
func TestDialBuildsClient(t *testing.T) {
	c, err := Dial("https://connector.internal:9443", true)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if c == nil {
		t.Fatal("Dial returned a nil client with no error")
	}
}
