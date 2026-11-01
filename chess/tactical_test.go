package chess

import (
	"math/rand/v2"
	"testing"
)

// filterCapturesAndPromotions is the independent oracle for the direct
// tactical generator: a full legal move list filtered down to forcing moves.
func filterCapturesAndPromotions(moves []Move) []Move {
	kept := moves[:0]
	for _, m := range moves {
		if _, promotion := m.Promotion(); m.IsCapture() || promotion {
			kept = append(kept, m)
		}
	}
	return kept
}

func requireTacticalMoveParity(t *testing.T, position *Position) {
	t.Helper()
	var buf [256]Move
	got := position.tacticalMoves(buf[:0])
	want := filterCapturesAndPromotions(position.LegalMoves())
	if len(got) != len(want) {
		t.Fatalf("on %q: generated %d tactical moves, want %d\ngot: %v\nwant: %v",
			position.FEN(), len(got), len(want), moveNames(got), moveNames(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("on %q: move %d = %s, want %s", position.FEN(), i, got[i], want[i])
		}
	}
}

func TestTacticalMovesMatchFilteredLegalMoves(t *testing.T) {
	fixtures := []string{
		NewStartingPosition().FEN(),
		kiwipeteFEN,
		freeQueenFEN,
		poisonedPawnFEN,
		enPassantQSFEN,
		promotionQSFEN,
		promotionCaptureFEN,
		inCheckRookFEN,
		inCheckQueenFEN,
		inCheckCaptureFEN,
		inCheckRankRookFEN,
		"4r1k1/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
		"4r1k1/8/8/8/8/8/4R3/4K3 w - - 0 1",
		"k6b/6P1/8/8/8/8/8/K7 w - - 0 1",
		"4r1k1/8/8/8/1b6/8/8/4K3 w - - 0 1",
		"4k3/8/8/3pP3/4K3/8/8/8 w - d6 0 1",
		"4k3/8/2p5/3r4/4K3/8/8/8 w - - 0 1",
	}
	for _, fen := range fixtures {
		requireTacticalMoveParity(t, mustFEN(t, fen))
	}
}

func TestTacticalMovesMatchFilteredLegalMovesInRandomGames(t *testing.T) {
	for seed := uint64(0); seed < 4; seed++ {
		rng := rand.New(rand.NewPCG(seed, seed+17))
		position := NewStartingPosition()
		for ply := 0; ply < 80; ply++ {
			requireTacticalMoveParity(t, position)
			moves := position.LegalMoves()
			if len(moves) == 0 {
				break
			}
			next := position.makeMove(moves[rng.IntN(len(moves))])
			position = &next
		}
	}
}
