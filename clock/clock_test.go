package clock_test

import (
	"sync"
	"testing"
	"time"

	"github.com/paper-board/sdk/clock"
)

func TestRealNowUTC(t *testing.T) {
	got := clock.Real{}.Now()
	if got.Location() != time.UTC {
		t.Fatalf("Real.Now() not UTC: %v", got.Location())
	}
	if time.Since(got) > 5*time.Second {
		t.Fatalf("Real.Now() too far in past: %v", got)
	}
}

func TestFakeSetAdvanceUTC(t *testing.T) {
	start := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	f := clock.NewFake(start)
	if got := f.Now(); !got.Equal(start) {
		t.Fatalf("initial: got %v, want %v", got, start)
	}
	f.Advance(2 * time.Hour)
	want := start.Add(2 * time.Hour)
	if got := f.Now(); !got.Equal(want) {
		t.Fatalf("after Advance: got %v, want %v", got, want)
	}
	f.Set(time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local))
	if got := f.Now(); got.Location() != time.UTC {
		t.Fatalf("Set should normalise to UTC, got %v", got.Location())
	}
}

func TestFakeConcurrentSafe(t *testing.T) {
	f := clock.NewFake(time.Unix(0, 0).UTC())
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); f.Advance(time.Second) }()
		go func() { defer wg.Done(); _ = f.Now() }()
	}
	wg.Wait()
	if got := f.Now(); got.Sub(time.Unix(0, 0).UTC()) != 100*time.Second {
		t.Fatalf("after 100 advances: got %v", got)
	}
}
