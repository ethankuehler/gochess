package chess

import (
	"context"
	"testing"
)

func TestNullMoveKeyAndClocks(t *testing.T) {
	fixtures := []string{
		NewStartingPosition().FEN(),
		"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		kiwipeteFEN,
		"4k3/8/8/8/8/8/8/4K3 b - - 12 40",
	}

	for _, fen := range fixtures {
		position := mustFEN(t, fen)
		null := position.nullMove()
		if null.SideToMove() == position.SideToMove() {
			t.Fatalf("%q: null move did not flip the side to move", fen)
		}
		if uint64(null.Key()) != null.computeKey() {
			t.Fatalf("%q: null key %d, recomputed %d", fen, null.Key(), null.computeKey())
		}
		if null.enPassantSquare != 0 || null.epHash != 0 {
			t.Fatalf("%q: null move kept the en passant target", fen)
		}
		if null.halfmoveClock != position.halfmoveClock+1 {
			t.Fatalf("%q: halfmove clock = %d, want %d", fen, null.halfmoveClock, position.halfmoveClock+1)
		}
		parsed := mustFEN(t, null.FEN())
		if parsed.Key() != null.Key() || parsed.SideToMove() != null.SideToMove() {
			t.Fatalf("%q: null FEN %q does not round-trip", fen, null.FEN())
		}
	}
}

func TestNullMoveGuard(t *testing.T) {
	middlegame := mustFEN(t, kiwipeteFEN)
	pawnEnding := mustFEN(t, "8/8/8/4k3/4p3/4P3/8/4K3 w - - 0 1")
	s := &searcher{opts: searchOptions{nullMove: true}}

	if !s.nullMoveAllowed(middlegame, 5, false, false, true, 0, 1) {
		t.Fatal("null move rejected in a scout middlegame node")
	}
	if s.nullMoveAllowed(middlegame, 5, true, false, true, 0, 1) {
		t.Fatal("null move allowed at a PV node")
	}
	if s.nullMoveAllowed(middlegame, 5, false, true, true, 0, 1) {
		t.Fatal("null move allowed while in check")
	}
	if s.nullMoveAllowed(middlegame, 4, false, false, true, 0, 1) {
		t.Fatal("null move allowed below depth 5")
	}
	if s.nullMoveAllowed(middlegame, 5, false, false, false, 0, 1) {
		t.Fatal("null move allowed twice in a row")
	}
	if s.nullMoveAllowed(middlegame, 5, false, false, true, mateScore-10, mateScore-5) {
		t.Fatal("null move allowed inside a mate window")
	}
	if s.nullMoveAllowed(pawnEnding, 5, false, false, true, 0, 1) {
		t.Fatal("null move allowed without non-pawn material")
	}
	s.opts.nullMove = false
	if s.nullMoveAllowed(middlegame, 5, false, false, true, 0, 1) {
		t.Fatal("null move allowed with the feature disabled")
	}
}

func TestNullMoveReducesNodes(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	limits := SearchLimits{MaxDepth: 6}
	without := position.searchWithOptions(context.Background(), limits, nil, searchOptions{pvs: true})
	with := position.searchWithOptions(context.Background(), limits, nil, searchOptions{pvs: true, nullMove: true})

	if with.Nodes >= without.Nodes {
		t.Fatalf("null move visited %d nodes, plain %d", with.Nodes, without.Nodes)
	}
	if with.Score != without.Score || with.BestMove != without.BestMove {
		t.Fatalf("null move changed the result: %s/%d vs %s/%d",
			with.BestMove, with.Score, without.BestMove, without.Score)
	}
}

func TestNullMoveKeepsMateDistance(t *testing.T) {
	position := mustFEN(t, "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 0 1")
	for depth := 1; depth <= 4; depth++ {
		result := position.Search(depth)
		if result.Score != MateScore-1 {
			t.Fatalf("depth %d: score = %d, want %d", depth, result.Score, MateScore-1)
		}
	}
}
