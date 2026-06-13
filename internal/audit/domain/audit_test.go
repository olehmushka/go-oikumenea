package domain

import (
	"errors"
	"testing"
)

// validActionRID is a native UUIDv8 whose packed kind nibble is 3 (action) — byte 6 = 0x83.
const validActionRID = "0192f3a1-0000-8300-8000-000000000000"

func validPerson() Entry {
	return Entry{
		ID:            validActionRID,
		ActorType:     ActorPerson,
		ActorPersonID: "0192f3a1-0000-8101-8601-000000000000", // person RID (any non-empty value)
		Action:        "assignment.grant",
		TargetType:    "role_assignment",
		RequestID:     "req-1",
		Outcome:       OutcomeSuccess,
	}
}

func TestValidateAcceptsWellFormedActors(t *testing.T) {
	if err := validPerson().Validate(); err != nil {
		t.Fatalf("valid person entry rejected: %v", err)
	}
	sys := validPerson()
	sys.ActorType = ActorSystem
	sys.ActorPersonID = ""
	sys.Subsystem = "bootstrap"
	if err := sys.Validate(); err != nil {
		t.Fatalf("valid system entry rejected: %v", err)
	}
}

func TestValidateRejectsBadEntries(t *testing.T) {
	cases := map[string]func(*Entry){
		"non-action RID":        func(e *Entry) { e.ID = "0192f3a1-0000-8100-8000-000000000000" }, // kind=1 (object)
		"person without id":     func(e *Entry) { e.ActorPersonID = "" },
		"person with subsystem": func(e *Entry) { e.Subsystem = "bootstrap" },
		"missing action":        func(e *Entry) { e.Action = "" },
		"missing target type":   func(e *Entry) { e.TargetType = "" },
		"missing request id":    func(e *Entry) { e.RequestID = "" },
		"unknown outcome":       func(e *Entry) { e.Outcome = "maybe" },
		"unknown actor type":    func(e *Entry) { e.ActorType = "robot" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := validPerson()
			mutate(&e)
			err := e.Validate()
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !errors.Is(err, ErrInvalidEntry) {
				t.Fatalf("error should wrap ErrInvalidEntry, got %v", err)
			}
		})
	}
}

func TestValidateRejectsSystemMisshape(t *testing.T) {
	e := validPerson()
	e.ActorType = ActorSystem // still has ActorPersonID set, no subsystem
	if err := e.Validate(); err == nil {
		t.Fatal("system actor with a person id and no subsystem should be rejected")
	}
}
