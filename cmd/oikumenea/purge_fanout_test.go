package main

import (
	"os"
	"strings"
	"testing"
)

// expectedPersonFanoutSubscriptions is the current width of the person purge/merge fan-out: the number
// of `atomic` subscribers attached to the person events bus in main.go (SubscribePersonPurge, which
// erases a module's person-owned rows in the purge transaction, plus SubscribePersonEvents, which
// re-homes rows on a provisional→canonical merge). Every one of these widens the single purge/merge
// transaction (D-EventOutbox, patterns.md "Domain events: atomic vs. notify").
//
// This is deliberately a hand-maintained number (review R-24): a new module joining the fan-out MUST
// move it, which forces (a) bumping this const, (b) updating the width baseline in patterns.md, and
// (c) a conscious check that the new atomic subscriber is the right call — the decision gate the fan-out
// had grown past invisibly (M31–M45 each added subscribers with no number moving).
const expectedPersonFanoutSubscriptions = 18

// TestPersonFanoutWidthGuard fails when the wired person purge/merge fan-out grows (or shrinks) without
// the baseline being updated — the automated half of R-24's width tracking (the SQL-cost half is
// TestPurgeWidthBudget in internal/person). It reads main.go directly (no DB), like the rewrap
// registry guard.
func TestPersonFanoutWidthGuard(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	got := strings.Count(string(body), ".SubscribePersonPurge(bus)") +
		strings.Count(string(body), ".SubscribePersonEvents(bus)")
	if got != expectedPersonFanoutSubscriptions {
		t.Fatalf("person purge/merge fan-out width = %d, expected %d.\n"+
			"A module joined or left the fan-out. This is a decision-level change (D-EventOutbox): update "+
			"expectedPersonFanoutSubscriptions AND the width baseline in docs/architecture/patterns.md, and "+
			"confirm the new atomic subscriber must roll back with the purge/merge transaction.", got, expectedPersonFanoutSubscriptions)
	}
}
