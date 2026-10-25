//go:build metrics

package chess

import (
	"slices"
	"testing"
)

func TestLegalMovesWithStatsMatchesLegalMoves(t *testing.T) {
	fens := []string{
		NewStartingPosition().FEN(),
		kiwipeteFEN,
		"7k/6Q1/6K1/8/8/8/8/8 b - - 0 1",
	}
	for _, fen := range fens {
		position := mustFEN(t, fen)
		want := position.LegalMoves()
		var stats moveGenStats
		got := position.legalMovesWithStats(nil, &stats)
		if !slices.Equal(got, want) {
			t.Fatalf("%s: instrumented moves = %v, want %v", fen, moveNames(got), moveNames(want))
		}
		if stats.legalMoves != uint64(len(want)) {
			t.Errorf("%s: legalMoves stat = %d, want %d", fen, stats.legalMoves, len(want))
		}
		if stats.pseudoMoves < stats.legalMoves {
			t.Errorf("%s: pseudoMoves = %d, legalMoves = %d", fen, stats.pseudoMoves, stats.legalMoves)
		}
		if stats.makeMoveCalls != stats.pseudoMoves {
			t.Errorf("%s: makeMoveCalls = %d, pseudoMoves = %d", fen, stats.makeMoveCalls, stats.pseudoMoves)
		}
		if stats.generationNS <= 0 || stats.filterNS <= 0 {
			t.Errorf("%s: durations = %d/%d", fen, stats.generationNS, stats.filterNS)
		}
	}
}
