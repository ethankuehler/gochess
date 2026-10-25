package chess

import "testing"

const (
	kiwipeteFEN     = "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1"
	freeQueenFEN    = "4k3/8/8/3q4/8/8/3R4/4K3 w - - 0 1"
	poisonedPawnFEN = "8/8/8/8/4k3/4p3/8/4Q1K1 w - - 0 1"
)

func TestSearchFindsMateInOne(t *testing.T) {
	position := mustFEN(t, "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 0 1")

	result := position.Search(1)
	if !result.HasMove {
		t.Fatal("search found no move in a mate-in-one position")
	}
	if got := result.BestMove.String(); got != "a1a8" {
		t.Fatalf("best move = %s, want a1a8", got)
	}
	if result.Score != MateScore-1 {
		t.Errorf("score = %d, want %d", result.Score, MateScore-1)
	}
	moves, ok := ScoreToMate(result.Score)
	if !ok || moves != 1 {
		t.Errorf("ScoreToMate(%d) = %d, %v; want 1, true", result.Score, moves, ok)
	}
}

func TestSearchCheckmate(t *testing.T) {
	result := mustFEN(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1").Search(3)

	if result.HasMove {
		t.Fatalf("checkmated position returned move %s", result.BestMove)
	}
	if result.Score != -MateScore {
		t.Errorf("score = %d, want %d", result.Score, -MateScore)
	}
}

func TestSearchStalemate(t *testing.T) {
	result := mustFEN(t, "7k/5Q2/6K1/8/8/8/8/8 b - - 0 1").Search(3)

	if result.HasMove {
		t.Fatalf("stalemated position returned move %s", result.BestMove)
	}
	if result.Score != 0 {
		t.Errorf("score = %d, want 0", result.Score)
	}
}

func TestSearchWinsFreeQueen(t *testing.T) {
	result := mustFEN(t, freeQueenFEN).Search(2)

	if !result.HasMove {
		t.Fatal("search found no move")
	}
	if got := result.BestMove.String(); got != "d2d5" {
		t.Fatalf("best move = %s, want d2d5", got)
	}
}

func TestSearchAvoidsPoisonedPawn(t *testing.T) {
	position := mustFEN(t, poisonedPawnFEN)

	shallow := position.Search(1)
	if got := shallow.BestMove.String(); got != "e1e3" {
		t.Fatalf("depth 1 best move = %s, want e1e3", got)
	}

	deep := position.Search(2)
	if deep.BestMove.String() == "e1e3" {
		t.Fatalf("depth 2 still plays the poisoned pawn capture e1e3")
	}
}

func TestSearchBestMoveIsLegal(t *testing.T) {
	fixtures := []string{
		NewStartingPosition().FEN(),
		kiwipeteFEN,
		freeQueenFEN,
		poisonedPawnFEN,
		"8/8/8/4k3/8/8/3Q4/4K3 w - - 0 1",
	}

	for _, fen := range fixtures {
		position := mustFEN(t, fen)
		for depth := 1; depth <= 3; depth++ {
			result := position.Search(depth)
			if !result.HasMove {
				t.Fatalf("depth %d on %q: no move", depth, fen)
			}
			if _, ok := findMove(position.LegalMoves(), result.BestMove.String()); !ok {
				t.Fatalf("depth %d on %q: best move %s is not legal", depth, fen, result.BestMove)
			}
			if _, err := position.ApplyMove(result.BestMove); err != nil {
				t.Fatalf("depth %d on %q: applying %s: %v", depth, fen, result.BestMove, err)
			}
		}
	}
}

func TestSearchDoesNotMutatePosition(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	before := position.FEN()

	position.Search(3)

	if after := position.FEN(); after != before {
		t.Fatalf("search mutated the position: before %q, after %q", before, after)
	}
}

func TestSearchDeterministic(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)

	first := position.Search(3)
	second := position.Search(3)

	if first.BestMove != second.BestMove || first.Score != second.Score || first.Nodes != second.Nodes {
		t.Fatalf("search is not deterministic: first %+v, second %+v", first, second)
	}
}

func TestSearchDepthZeroPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Search(0) did not panic")
		}
	}()

	mustFEN(t, NewStartingPosition().FEN()).Search(0)
}

func TestScoreToMate(t *testing.T) {
	cases := []struct {
		score int
		moves int
		ok    bool
	}{
		{0, 0, false},
		{100, 0, false},
		{MateScore - 200, 0, false},
		{MateScore - 1, 1, true},
		{MateScore - 3, 2, true},
		{-(MateScore - 2), -1, true},
		{-MateScore, 0, true},
	}

	for _, tc := range cases {
		moves, ok := ScoreToMate(tc.score)
		if moves != tc.moves || ok != tc.ok {
			t.Errorf("ScoreToMate(%d) = %d, %v; want %d, %v", tc.score, moves, ok, tc.moves, tc.ok)
		}
	}
}

func TestOrderMoves(t *testing.T) {
	position := mustFEN(t, freeQueenFEN)
	moves := position.LegalMoves()

	var scratch [256]Move
	ordered := orderMoves(moves, scratch[:0], Move{})
	if len(ordered) != len(moves) {
		t.Fatalf("ordered %d moves, want %d", len(ordered), len(moves))
	}

	counts := make(map[string]int, len(moves))
	for _, m := range moves {
		counts[m.String()]++
	}
	for _, m := range ordered {
		counts[m.String()]--
	}
	for uci, count := range counts {
		if count != 0 {
			t.Errorf("move %s count = %d after ordering, want 0", uci, count)
		}
	}

	sawQuiet := false
	for _, m := range ordered {
		_, promotion := m.Promotion()
		if !m.IsCapture() && !promotion {
			sawQuiet = true
			continue
		}
		if sawQuiet {
			t.Fatalf("capture or promotion %s comes after a quiet move", m)
		}
	}
}

func TestAlphaBetaMatchesMinimax(t *testing.T) {
	fixtures := []string{
		NewStartingPosition().FEN(),
		kiwipeteFEN,
		poisonedPawnFEN,
		"8/8/8/4k3/8/8/3Q4/4K3 w - - 0 1",
	}

	for _, fen := range fixtures {
		position := mustFEN(t, fen)
		for depth := 1; depth <= 3; depth++ {
			want := referenceMinimax(position, depth, 0)
			got := position.Search(depth).Score
			if got != want {
				t.Errorf("depth %d on %q: alpha-beta score = %d, minimax = %d", depth, fen, got, want)
			}
		}
	}
}

// referenceMinimax is a full-width minimax with the same evaluation and terminal
// rules as Search. It exists only to validate alpha-beta pruning in tests.
func referenceMinimax(b *Position, depth, ply int) int {
	if b.halfmoveClock >= 100 {
		return 0
	}

	var buf [256]Move
	if depth == 0 {
		if b.InCheck() && len(b.legalMoves(buf[:0])) == 0 {
			return -mateScore + ply
		}
		return b.evaluateForSideToMove()
	}

	moves := b.legalMoves(buf[:0])
	if len(moves) == 0 {
		if b.InCheck() {
			return -mateScore + ply
		}
		return 0
	}

	best := -infinity
	for _, m := range moves {
		child := b.makeMove(m)
		score := -referenceMinimax(&child, depth-1, ply+1)
		if score > best {
			best = score
		}
	}
	return best
}
