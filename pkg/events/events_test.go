package events

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

type testEvent struct {
	typ string
	val string
}

func (e testEvent) Type() string { return e.typ }

func TestPublishDispatchesInOrder(t *testing.T) {
	bus := NewBus()
	var order []string
	bus.Subscribe("a", func(_ context.Context, _ pgx.Tx, e Event) error {
		order = append(order, "first:"+e.(testEvent).val)
		return nil
	})
	bus.Subscribe("a", func(_ context.Context, _ pgx.Tx, e Event) error {
		order = append(order, "second:"+e.(testEvent).val)
		return nil
	})
	if err := bus.Publish(context.Background(), nil, testEvent{typ: "a", val: "x"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(order) != 2 || order[0] != "first:x" || order[1] != "second:x" {
		t.Fatalf("handlers ran out of order: %v", order)
	}
}

func TestPublishStopsOnFirstError(t *testing.T) {
	bus := NewBus()
	boom := errors.New("boom")
	ran := 0
	bus.Subscribe("a", func(_ context.Context, _ pgx.Tx, _ Event) error {
		ran++
		return boom
	})
	bus.Subscribe("a", func(_ context.Context, _ pgx.Tx, _ Event) error {
		ran++ // must NOT run — the first handler aborted dispatch
		return nil
	})
	err := bus.Publish(context.Background(), nil, testEvent{typ: "a"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if ran != 1 {
		t.Fatalf("ran = %d handlers, want 1 (dispatch should stop at the error)", ran)
	}
}

func TestPublishNoSubscribersIsNoop(t *testing.T) {
	bus := NewBus()
	if err := bus.Publish(context.Background(), nil, testEvent{typ: "unknown"}); err != nil {
		t.Fatalf("publish with no subscribers: %v", err)
	}
}

func TestSubscribersAreTypeScoped(t *testing.T) {
	bus := NewBus()
	hit := false
	bus.Subscribe("a", func(_ context.Context, _ pgx.Tx, _ Event) error { hit = true; return nil })
	if err := bus.Publish(context.Background(), nil, testEvent{typ: "b"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if hit {
		t.Fatal("handler for type \"a\" ran on a type \"b\" event")
	}
}
