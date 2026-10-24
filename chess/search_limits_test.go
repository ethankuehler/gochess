package chess

import (
	"context"
	"testing"
	"time"
)

func legalMoveFor(t *testing.T, position *Position, uci string) Move {
	t.Helper()
	parsed, err := ParseUCIMove(uci)
	if err != nil {
		t.Fatalf("ParseUCIMove(%q): %v", uci, err)
	}
	legal, ok := position.FindLegalMove(*parsed)
	if !ok {
		t.Fatalf("move %q is not legal in %s", uci, position.FEN())
	}
	return legal
}

func assertPVLegal(t *testing.T, position *Position, pv []Move) {
	t.Helper()
	current := position
	for i, move := range pv {
		next, err := current.ApplyMove(move)
		if err != nil {
			t.Fatalf("PV move %d (%s) is not legal: %v", i+1, move, err)
		}
		current = next
	}
}

func TestSearchWithLimitsIterativeDeepening(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)

	var infos []SearchInfo
	result := position.SearchWithLimits(context.Background(), SearchLimits{MaxDepth: 3}, func(info SearchInfo) {
		infos = append(infos, info)
	})

	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if result.Depth != 3 {
		t.Fatalf("completed depth = %d, want 3", result.Depth)
	}
	if len(infos) != 3 {
		t.Fatalf("received %d info updates, want 3", len(infos))
	}
	for i, info := range infos {
		if info.Depth != i+1 {
			t.Errorf("info %d depth = %d, want %d", i, info.Depth, i+1)
		}
		if info.SelDepth < info.Depth {
			t.Errorf("info %d seldepth = %d, want at least depth %d", i, info.SelDepth, info.Depth)
		}
		if i > 0 && info.Nodes < infos[i-1].Nodes {
			t.Errorf("info %d nodes = %d, decreased from %d", i, info.Nodes, infos[i-1].Nodes)
		}
	}
	if len(result.PV) == 0 {
		t.Fatal("result has no PV")
	}
	if result.PV[0] != result.BestMove {
		t.Fatalf("PV starts with %s, best move is %s", result.PV[0], result.BestMove)
	}
	assertPVLegal(t, position, result.PV)
	if len(result.PV) >= 2 && result.PonderMove != result.PV[1] {
		t.Fatalf("ponder move = %s, want %s", result.PonderMove, result.PV[1])
	}
}

func TestSearchWithLimitsSoftDeadline(t *testing.T) {
	position := NewStartingPosition()
	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth:     10,
		SoftDeadline: time.Now().Add(-time.Millisecond),
	}, nil)

	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if result.Depth < 1 {
		t.Fatalf("completed depth = %d, want at least 1", result.Depth)
	}
	if result.Depth >= 10 {
		t.Fatalf("search reached depth %d despite an expired soft deadline", result.Depth)
	}
	if _, ok := position.FindLegalMove(result.BestMove); !ok {
		t.Fatalf("best move %s is not legal", result.BestMove)
	}
}

func TestSearchWithLimitsNodeLimit(t *testing.T) {
	position := NewStartingPosition()
	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth: 20,
		MaxNodes: 500,
	}, nil)

	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if result.Nodes > 500 {
		t.Fatalf("visited %d nodes, limit was 500", result.Nodes)
	}
	if _, ok := position.FindLegalMove(result.BestMove); !ok {
		t.Fatalf("best move %s is not legal", result.BestMove)
	}
	assertPVLegal(t, position, result.PV)
}

func TestSearchWithLimitsCancelledBeforeFirstIteration(t *testing.T) {
	position := NewStartingPosition()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := position.SearchWithLimits(ctx, SearchLimits{MaxDepth: 5}, nil)
	if !result.HasMove {
		t.Fatal("cancelled search returned no fallback move")
	}
	if result.Depth != 0 {
		t.Fatalf("completed depth = %d, want 0", result.Depth)
	}
	if result.Nodes != 0 {
		t.Fatalf("nodes = %d, want 0", result.Nodes)
	}
	if len(result.PV) != 1 || result.PV[0] != result.BestMove {
		t.Fatalf("fallback PV = %v, best move = %s", result.PV, result.BestMove)
	}
	if _, ok := position.FindLegalMove(result.BestMove); !ok {
		t.Fatalf("fallback move %s is not legal", result.BestMove)
	}
}

func TestSearchWithLimitsRootMoves(t *testing.T) {
	position := NewStartingPosition()
	e2e4 := legalMoveFor(t, position, "e2e4")

	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth:  2,
		RootMoves: []Move{e2e4},
	}, nil)

	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if result.BestMove != e2e4 {
		t.Fatalf("best move = %s, want e2e4", result.BestMove)
	}
	if len(result.PV) == 0 || result.PV[0] != e2e4 {
		t.Fatalf("PV = %v, want it to start with e2e4", result.PV)
	}
}

func TestSearchWithLimitsEmptyRootMoves(t *testing.T) {
	position := NewStartingPosition()
	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth:  2,
		RootMoves: []Move{},
	}, nil)

	if result.HasMove {
		t.Fatalf("search returned move %s with an empty root list", result.BestMove)
	}
}

func TestSearchWithLimitsIllegalRootMoves(t *testing.T) {
	position := NewStartingPosition()
	parsed, err := ParseUCIMove("e7e5")
	if err != nil {
		t.Fatalf("ParseUCIMove: %v", err)
	}

	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth:  2,
		RootMoves: []Move{*parsed},
	}, nil)
	if result.HasMove {
		t.Fatalf("search used illegal root move %s", result.BestMove)
	}
}

func TestSearchWithLimitsMatePV(t *testing.T) {
	position := mustFEN(t, "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 0 1")
	result := position.SearchWithLimits(context.Background(), SearchLimits{MaxDepth: 3}, nil)

	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if got := result.BestMove.String(); got != "a1a8" {
		t.Fatalf("best move = %s, want a1a8", got)
	}
	if result.Score != MateScore-1 {
		t.Fatalf("score = %d, want %d", result.Score, MateScore-1)
	}
	if len(result.PV) == 0 || result.PV[0].String() != "a1a8" {
		t.Fatalf("PV = %v, want it to start with a1a8", result.PV)
	}
	assertPVLegal(t, position, result.PV)
}

func TestSearchWithLimitsTerminalPositions(t *testing.T) {
	checkmate := mustFEN(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	result := checkmate.SearchWithLimits(context.Background(), SearchLimits{MaxDepth: 3}, nil)
	if result.HasMove {
		t.Fatalf("checkmated position returned move %s", result.BestMove)
	}
	if result.Score != -MateScore {
		t.Fatalf("checkmate score = %d, want %d", result.Score, -MateScore)
	}

	stalemate := mustFEN(t, "7k/5Q2/6K1/8/8/8/8/8 b - - 0 1")
	result = stalemate.SearchWithLimits(context.Background(), SearchLimits{MaxDepth: 3}, nil)
	if result.HasMove {
		t.Fatalf("stalemated position returned move %s", result.BestMove)
	}
	if result.Score != 0 {
		t.Fatalf("stalemate score = %d, want 0", result.Score)
	}
}

func TestSearchWithLimitsDoesNotMutatePosition(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	before := position.FEN()

	position.SearchWithLimits(context.Background(), SearchLimits{MaxDepth: 3}, nil)

	if after := position.FEN(); after != before {
		t.Fatalf("search mutated the position: before %q, after %q", before, after)
	}
}

func TestSearchWithLimitsNoInfoAfterCancellation(t *testing.T) {
	position := NewStartingPosition()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	result := position.SearchWithLimits(ctx, SearchLimits{MaxDepth: 5}, func(SearchInfo) {
		called = true
	})
	if called {
		t.Fatal("info callback ran for a search cancelled before the first iteration")
	}
	if !result.HasMove {
		t.Fatal("no fallback move after cancellation")
	}
}
