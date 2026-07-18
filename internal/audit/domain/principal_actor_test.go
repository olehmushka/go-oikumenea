package domain

import (
	"strings"
	"testing"
)

// A service principal is a `system` actor that NAMES ITSELF (M51 / D-ServiceIdentities) — not a third
// actor kind, because D-Audit's two kinds are binding and the DB CHECK enforces them. These tests
// mirror the audit_log_actor_shape constraint in Go so a bad Entry fails before it reaches Postgres.

func validSystemEntry() Entry {
	return Entry{
		ID:         validActionRID, // shared fixture: a UUIDv8 whose packed kind nibble is action
		ActorType:  ActorSystem,
		Subsystem:  "data-import",
		Action:     "import.apply",
		TargetType: "country",
		RequestID:  "req-1",
		Outcome:    OutcomeSuccess,
	}
}

func TestSystemActorMayNamePrincipal(t *testing.T) {
	e := validSystemEntry()
	e.ActorPrincipalID = "01890000-0000-8000-8000-000000000001"
	if err := e.Validate(); err != nil {
		t.Fatalf("system entry naming its principal rejected: %v", err)
	}
}

// A system action with no machine caller (bootstrap, event subscribers, the pinax autoseeder) leaves
// the field empty — it is optional, not required.
func TestSystemActorPrincipalIsOptional(t *testing.T) {
	if err := validSystemEntry().Validate(); err != nil {
		t.Fatalf("system entry without a principal rejected: %v", err)
	}
}

// The person arm must never carry a principal: a human acting is not a machine acting, and the DB
// CHECK forbids the combination. Catching it here gives a typed error instead of a constraint 500.
func TestPersonActorRejectsPrincipal(t *testing.T) {
	e := Entry{
		ID:            validActionRID,
		ActorType:     ActorPerson,
		ActorPersonID: "01890000-0000-8000-8000-0000000000ff",
		Action:        "person.update",
		TargetType:    "person",
		RequestID:     "req-2",
		Outcome:       OutcomeSuccess,
	}
	e.ActorPrincipalID = "01890000-0000-8000-8000-000000000001"
	err := e.Validate()
	if err == nil {
		t.Fatal("person actor accepted an actorPrincipalId; the DB CHECK forbids it")
	}
	if !strings.Contains(err.Error(), "actorPrincipalId") {
		t.Errorf("error %q does not name the offending field", err)
	}
}
