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
		if stats.makeMoveCalls > stats.pseudoMoves {
			t.Errorf("%s: makeMoveCalls = %d, pseudoMoves = %d", fen, stats.makeMoveCalls, stats.pseudoMoves)
		}
		if stats.generationNS <= 0 || stats.filterNS < 0 {
			t.Errorf("%s: durations = %d/%d", fen, stats.generationNS, stats.filterNS)
		}
	}
}

func TestLegalMovesWithStatsCopyMakeCounts(t *testing.T) {
	t.Run("starting position needs no legality copies", func(t *testing.T) {
		var stats moveGenStats
		moves := NewStartingPosition().legalMovesWithStats(nil, &stats)
		if stats.pseudoMoves != 20 || stats.legalMoves != 20 || len(moves) != 20 || stats.makeMoveCalls != 0 || stats.filterNS != 0 {
			t.Fatalf("starting position stats = %+v, moves = %d", stats, len(moves))
		}
	})
	t.Run("en passant is copy validated", func(t *testing.T) {
		position := mustFEN(t, "4k3/8/8/K2pP2r/8/8/8/8 w - d6 0 1")
		var stats moveGenStats
		moves := position.legalMovesWithStats(nil, &stats)
		if stats.makeMoveCalls == 0 || stats.filterNS <= 0 {
			t.Fatalf("en-passant stats = %+v", stats)
		}
		if stats.pseudoMoves <= stats.legalMoves {
			t.Fatalf("rejected special move was not counted: %+v", stats)
		}
		requireNoMoves(t, moves, "e5d6")
	})
}
