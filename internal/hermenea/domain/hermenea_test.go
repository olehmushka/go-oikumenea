package domain

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	base, max := time.Second, 30*time.Second
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{1, time.Second},       // base
		{2, 2 * time.Second},   // base*2
		{3, 4 * time.Second},   // base*4
		{4, 8 * time.Second},   // base*8
		{10, 30 * time.Second}, // capped at max
		{0, time.Second},       // floor at 1 attempt
	}
	for _, c := range cases {
		if got := Backoff(c.attempts, base, max); got != c.want {
			t.Errorf("Backoff(%d) = %v, want %v", c.attempts, got, c.want)
		}
	}
}

func TestScheduleInterval(t *testing.T) {
	ok := map[string]time.Duration{
		"@hourly":      time.Hour,
		"@daily":       24 * time.Hour,
		"@weekly":      7 * 24 * time.Hour,
		"@every 30m":   30 * time.Minute,
		"@every 1h30m": 90 * time.Minute,
	}
	for spec, want := range ok {
		got, err := ScheduleInterval(spec)
		if err != nil || got != want {
			t.Errorf("ScheduleInterval(%q) = %v, %v; want %v", spec, got, err, want)
		}
	}
	for _, bad := range []string{"", "* * * * *", "@every", "@every -1s", "@yearly", "30m"} {
		if _, err := ScheduleInterval(bad); err == nil {
			t.Errorf("ScheduleInterval(%q): expected error", bad)
		}
	}
}

func TestScheduleDue(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	if !ScheduleDue(time.Hour, nil, now) {
		t.Error("never-enqueued schedule should be due")
	}
	recent := now.Add(-30 * time.Minute)
	if ScheduleDue(time.Hour, &recent, now) {
		t.Error("schedule enqueued 30m ago with 1h interval should not be due")
	}
	old := now.Add(-90 * time.Minute)
	if !ScheduleDue(time.Hour, &old, now) {
		t.Error("schedule enqueued 90m ago with 1h interval should be due")
	}
}
