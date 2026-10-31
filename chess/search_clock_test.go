package chess

import (
	"context"
	"testing"
	"time"
)

func TestSearchClockDeadlinesRoundTrip(t *testing.T) {
	var clock SearchClock
	soft, hard := clock.Deadlines()
	if !soft.IsZero() || !hard.IsZero() {
		t.Fatalf("zero clock deadlines = %v, %v; want zero times", soft, hard)
	}

	soft = time.Unix(1_000, 123)
	hard = time.Unix(2_000, 456)
	clock.SetDeadlines(soft, hard)
	gotSoft, gotHard := clock.Deadlines()
	if gotSoft.UnixNano() != soft.UnixNano() || gotHard.UnixNano() != hard.UnixNano() {
		t.Fatalf("deadlines = %v, %v; want %v, %v", gotSoft, gotHard, soft, hard)
	}

	clock.SetDeadlines(time.Time{}, hard)
	gotSoft, gotHard = clock.Deadlines()
	if !gotSoft.IsZero() {
		t.Fatalf("soft deadline = %v, want zero", gotSoft)
	}
	if gotHard.UnixNano() != hard.UnixNano() {
		t.Fatalf("hard deadline = %v, want %v", gotHard, hard)
	}
}

func TestSearchWithLimitsClockOverridesLimits(t *testing.T) {
	position := NewStartingPosition()
	clock := &SearchClock{}
	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth:     2,
		Deadline:     time.Now().Add(-time.Second),
		SoftDeadline: time.Now().Add(-time.Second),
		Clock:        clock,
	}, nil)

	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if result.Depth != 2 {
		t.Fatalf("completed depth = %d, want 2; a non-nil clock must override the time fields", result.Depth)
	}
}

func TestSearchWithLimitsClockHardDeadline(t *testing.T) {
	position := NewStartingPosition()
	clock := &SearchClock{}
	clock.SetDeadlines(time.Time{}, time.Now().Add(-time.Millisecond))

	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth: 5,
		Clock:    clock,
	}, nil)

	if !result.HasMove {
		t.Fatal("search returned no fallback move")
	}
	if result.Depth != 0 {
		t.Fatalf("completed depth = %d, want 0 with an expired hard deadline", result.Depth)
	}
	if _, ok := position.FindLegalMove(result.BestMove); !ok {
		t.Fatalf("fallback move %s is not legal", result.BestMove)
	}
}

func TestSearchWithLimitsClockSoftDeadline(t *testing.T) {
	position := NewStartingPosition()
	clock := &SearchClock{}
	clock.SetDeadlines(time.Now().Add(-time.Millisecond), time.Time{})

	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth: 5,
		Clock:    clock,
	}, nil)

	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if result.Depth != 1 {
		t.Fatalf("completed depth = %d, want 1 with an expired soft deadline", result.Depth)
	}
}

func TestSearchClockUpdateSoftDuringSearch(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	clock := &SearchClock{}

	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth: 8,
		Clock:    clock,
	}, func(info SearchInfo) {
		if info.Depth == 2 {
			// This mirrors ponderhit: the deadline is installed after the
			// search has already started, between iterations.
			clock.SetDeadlines(time.Now().Add(-time.Millisecond), time.Time{})
		}
	})

	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if result.Depth != 2 {
		t.Fatalf("completed depth = %d, want 2 after the soft deadline was installed", result.Depth)
	}
}

func TestSearchClockUpdateDuringSearch(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	clock := &SearchClock{}
	reached := make(chan struct{})
	updated := make(chan struct{})

	go func() {
		<-reached
		clock.SetDeadlines(time.Time{}, time.Now().Add(-time.Millisecond))
		close(updated)
	}()

	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth: 10,
		Clock:    clock,
	}, func(info SearchInfo) {
		if info.Depth == 2 {
			select {
			case <-reached:
			default:
				close(reached)
			}
		}
	})
	<-updated

	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if result.Depth < 2 {
		t.Fatalf("completed depth = %d, want at least 2", result.Depth)
	}
	if result.Depth >= 10 {
		t.Fatalf("completed depth = %d despite a hard deadline installed mid-search", result.Depth)
	}
}
