package domain

import (
	"errors"
	"testing"
)

const validActionRID = "urn:oikumenea:audit:local:action__grant:0192f3a1-0000-7000-8000-000000000000"

func validPerson() Entry {
	return Entry{
		ID:            validActionRID,
		ActorType:     ActorPerson,
		ActorPersonID: "urn:oikumenea:person:local:person:abc",
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
		"non-action RID":        func(e *Entry) { e.ID = "urn:oikumenea:audit:local:auditentry:x" },
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
