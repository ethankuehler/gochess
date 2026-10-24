package chess

import (
	"context"
	"math/rand/v2"
	"testing"
)

// playUCI applies moves and returns the final position plus the keys of every
// position before it, oldest first.
func playUCI(t *testing.T, position *Position, moves ...string) (*Position, []PositionKey) {
	t.Helper()
	history := make([]PositionKey, 0, len(moves))
	for _, uci := range moves {
		history = append(history, position.Key())
		next, _, err := position.ApplyUCIMove(uci)
		if err != nil {
			t.Fatalf("ApplyUCIMove(%q): %v", uci, err)
		}
		position = next
	}
	return position, history
}

func TestPositionKeyIncrementalMatchesRecompute(t *testing.T) {
	cases := []struct {
		fen   string
		moves []string
	}{
		{
			fen:   NewStartingPosition().FEN(),
			moves: []string{"e2e4", "e7e5", "g1f3", "b8c6", "f1b5", "a7a6", "b5c6", "d7c6", "e1g1"},
		},
		{
			fen:   "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
			moves: []string{"e5d6"},
		},
		{
			fen:   "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
			moves: []string{"e1g1", "e8c8", "a1a2", "h8h2"},
		},
		{
			fen:   "1n6/P7/8/8/8/8/8/K6k w - - 0 1",
			moves: []string{"a7b8n"},
		},
	}

	for _, tc := range cases {
		position := mustFEN(t, tc.fen)
		if got, want := position.key, position.computeKey(); got != want {
			t.Fatalf("initial key for %q = %d, want %d", tc.fen, got, want)
		}
		for _, uci := range tc.moves {
			next, _, err := position.ApplyUCIMove(uci)
			if err != nil {
				t.Fatalf("%q: ApplyUCIMove(%q): %v", tc.fen, uci, err)
			}
			if got, want := next.key, next.computeKey(); got != want {
				t.Fatalf("%q after %q: incremental key = %d, want %d", tc.fen, uci, got, want)
			}
			position = next
		}
	}
}

func TestPositionKeyRandomPlay(t *testing.T) {
	fixtures := []string{
		NewStartingPosition().FEN(),
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"7k/P1p5/8/8/8/8/p1P5/K7 w - - 0 1",
	}

	for seed, fen := range fixtures {
		position := mustFEN(t, fen)
		rng := rand.New(rand.NewPCG(uint64(seed+1), uint64(seed+7)))
		for ply := 0; ply < 160; ply++ {
			moves := position.LegalMoves()
			if len(moves) == 0 {
				break
			}
			move := moves[rng.IntN(len(moves))]
			next := position.makeMove(move)
			if got, want := next.key, next.computeKey(); got != want {
				t.Fatalf("%q ply %d after %s: incremental key = %d, want %d", fen, ply+1, move, got, want)
			}
			position = &next
		}
	}
}

func TestPositionKeyEquality(t *testing.T) {
	base := mustFEN(t, "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1")
	same := mustFEN(t, "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1")
	if base.Key() != same.Key() {
		t.Fatal("identical positions have different keys")
	}

	withoutEP := mustFEN(t, "4k3/8/8/3pP3/8/8/8/4K3 w - - 0 1")
	if base.Key() == withoutEP.Key() {
		t.Error("a capturable en passant target did not change the key")
	}

	// With no pawn able to capture, FIDE treats the positions as the same.
	noCapture := mustFEN(t, "4k3/8/8/3p4/8/8/8/4K3 w - d6 0 1")
	noCaptureNoEP := mustFEN(t, "4k3/8/8/3p4/8/8/8/4K3 w - - 0 1")
	if noCapture.Key() != noCaptureNoEP.Key() {
		t.Error("an uncapturable en passant target changed the key")
	}

	blackToMove := mustFEN(t, "4k3/8/8/3pP3/8/8/8/4K3 b - d6 0 1")
	if base.Key() == blackToMove.Key() {
		t.Error("side to move did not change the key")
	}

	noCastling := mustFEN(t, "r3k2r/8/8/8/8/8/8/R3K2R w - - 0 1")
	castling := mustFEN(t, "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")
	if noCastling.Key() == castling.Key() {
		t.Error("castling rights did not change the key")
	}
}

func TestIsThreefoldRepetition(t *testing.T) {
	// A four-ply king shuffle returns to the starting position. Repeating it
	// twice makes the starting position the third occurrence.
	position, history := playUCI(t, mustFEN(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1"),
		"e1d1", "e8d8", "d1e1", "d8e8")
	if position.IsThreefoldRepetition(history) {
		t.Fatal("position was called a threefold repetition after only two occurrences")
	}

	position, history = playUCI(t, mustFEN(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1"),
		"e1d1", "e8d8", "d1e1", "d8e8",
		"e1d1", "e8d8", "d1e1", "d8e8")
	if !position.IsThreefoldRepetition(history) {
		t.Fatal("position was not called a threefold repetition after three occurrences")
	}
	if got := position.RepetitionCount(history); got != 2 {
		t.Fatalf("RepetitionCount = %d, want 2 earlier occurrences", got)
	}
}

func TestSearchDrawsAtRoot(t *testing.T) {
	// White is up a rook, so ordinary evaluation is clearly positive.
	position, history := playUCI(t, mustFEN(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1"),
		"e1d1", "e8d8", "d1e1", "d8e8",
		"e1d1", "e8d8", "d1e1", "d8e8")
	if !position.IsThreefoldRepetition(history) {
		t.Fatal("test position is not a threefold repetition")
	}

	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth:    1,
		GameHistory: history,
	}, nil)
	if !result.HasMove {
		t.Fatal("a drawn root returned no legal move")
	}
	if !result.Draw {
		t.Error("threefold root was not reported as a draw")
	}
	if result.Score != 0 {
		t.Errorf("threefold root score = %d, want 0", result.Score)
	}

	fifty := mustFEN(t, "4k3/8/8/8/8/8/8/R3K3 w - - 100 1")
	root := fifty.SearchWithLimits(context.Background(), SearchLimits{MaxDepth: 1}, nil)
	if !root.HasMove {
		t.Fatal("fifty-move root returned no legal move")
	}
	if !root.Draw || root.Score != 0 {
		t.Errorf("fifty-move root = draw %v score %d, want draw true score 0", root.Draw, root.Score)
	}
}

func TestSearchDetectsRepetitionInPath(t *testing.T) {
	position := mustFEN(t, "4k3/8/8/3q4/8/8/3R4/4K3 w - - 2 1")
	quiet := legalMoveFor(t, position, "d2d3")
	child, _, err := position.ApplyUCIMove("d2d3")
	if err != nil {
		t.Fatalf("ApplyUCIMove(d2d3): %v", err)
	}

	// Two earlier occurrences of the child plus the occurrence reached by
	// d2d3 make three. The search must score the line as a draw.
	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth:    1,
		RootMoves:   []Move{quiet},
		GameHistory: []PositionKey{child.Key(), child.Key()},
	}, nil)
	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if result.Score != 0 {
		t.Errorf("repeating child score = %d, want 0", result.Score)
	}
}

func TestSearchDetectsFiftyMoveInPath(t *testing.T) {
	// The quiet move takes the halfmove clock to 100, which is a draw.
	position := mustFEN(t, "4k3/8/8/3q4/8/8/3R4/4K3 w - - 99 1")
	quiet := legalMoveFor(t, position, "d2d3")

	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth:  1,
		RootMoves: []Move{quiet},
	}, nil)
	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if result.Score != 0 {
		t.Errorf("fifty-move child score = %d, want 0", result.Score)
	}
}

func TestSearchPrefersCheckmateAtFiftyMoveHorizon(t *testing.T) {
	// The quiet mating move takes the halfmove clock to 100, but checkmate
	// ends the game before the draw can be claimed.
	position := mustFEN(t, "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 99 1")

	result := position.Search(1)
	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if got := result.BestMove.String(); got != "a1a8" {
		t.Fatalf("best move = %s, want a1a8", got)
	}
	if result.Score != MateScore-1 {
		t.Errorf("score = %d, want mate score %d", result.Score, MateScore-1)
	}
	if result.Draw {
		t.Error("checkmate was reported as a draw")
	}
}

func TestSearchAvoidsRepetitionWhenAhead(t *testing.T) {
	// White is up a rook. Repeating into a third occurrence of the position
	// after a1b1 would be a draw, so the search must prefer a different move.
	position := mustFEN(t, "4k3/8/8/8/8/8/8/R3K3 w - - 2 1")
	repeat := legalMoveFor(t, position, "a1b1")
	other := legalMoveFor(t, position, "e1d1")
	child, _, err := position.ApplyUCIMove("a1b1")
	if err != nil {
		t.Fatalf("ApplyUCIMove(a1b1): %v", err)
	}

	result := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth:    1,
		RootMoves:   []Move{repeat, other},
		GameHistory: []PositionKey{child.Key(), child.Key()},
	}, nil)
	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if result.BestMove != other {
		t.Fatalf("best move = %s, want %s to avoid the repetition", result.BestMove, other)
	}
	if result.Score == 0 {
		t.Errorf("score = 0, want the winning rook evaluation")
	}
}
