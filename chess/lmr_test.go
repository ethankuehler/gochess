package chess

import (
	"context"
	"testing"
)

func TestLateMoveReductionGuard(t *testing.T) {
	queen := mustFEN(t, "4k3/8/8/8/8/8/8/3QK3 w - - 0 1")
	quiet := legalMoveFor(t, queen, "d1c2")
	check := legalMoveFor(t, queen, "d1d8")

	rook := mustFEN(t, freeQueenFEN)
	capture := legalMoveFor(t, rook, "d2d5")

	promoting := mustFEN(t, "4k3/6P1/8/8/8/8/8/4K3 w - - 0 1")
	promotion := legalMoveFor(t, promoting, "g7g8q")

	quietChild := queen.makeMove(quiet)
	checkChild := queen.makeMove(check)
	captureChild := rook.makeMove(capture)
	promotionChild := promoting.makeMove(promotion)

	s := &searcher{opts: searchOptions{lmr: true}}
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"late quiet", s.lateMoveReduction(&quietChild, quiet, 6, 4, false, 0, 1), 1},
		{"very late quiet", s.lateMoveReduction(&quietChild, quiet, 10, 12, false, 0, 1), 3},
		{"early quiet", s.lateMoveReduction(&quietChild, quiet, 6, 2, false, 0, 1), 0},
		{"shallow", s.lateMoveReduction(&quietChild, quiet, 2, 4, false, 0, 1), 0},
		{"pv node", s.lateMoveReduction(&quietChild, quiet, 6, 4, true, 0, 1), 0},
		{"capture", s.lateMoveReduction(&captureChild, capture, 6, 4, false, 0, 1), 0},
		{"promotion", s.lateMoveReduction(&promotionChild, promotion, 6, 4, false, 0, 1), 0},
		{"gives check", s.lateMoveReduction(&checkChild, check, 6, 4, false, 0, 1), 0},
		{"mate window", s.lateMoveReduction(&quietChild, quiet, 6, 4, false, mateScore-10, mateScore-5), 0},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: reduction = %d, want %d", tc.name, tc.got, tc.want)
		}
	}

	s.opts.lmr = false
	if got := s.lateMoveReduction(&quietChild, quiet, 6, 4, false, 0, 1); got != 0 {
		t.Fatalf("reduction = %d with lmr disabled, want 0", got)
	}
}

func TestLateMoveReductionStaysClose(t *testing.T) {
	fixtures := []string{
		NewStartingPosition().FEN(),
		kiwipeteFEN,
		freeQueenFEN,
		poisonedPawnFEN,
	}

	for _, fen := range fixtures {
		position := mustFEN(t, fen)
		for depth := 5; depth <= 6; depth++ {
			plain := position.searchWithOptions(context.Background(), SearchLimits{MaxDepth: depth}, nil, searchOptions{pvs: true})
			reduced := position.searchWithOptions(context.Background(), SearchLimits{MaxDepth: depth}, nil, searchOptions{pvs: true, lmr: true})

			if drop := plain.Score - reduced.Score; drop > 100 {
				t.Errorf("depth %d on %q: LMR score %d, plain %d", depth, fen, reduced.Score, plain.Score)
			}
			if _, err := position.ApplyMove(reduced.BestMove); err != nil {
				t.Errorf("depth %d on %q: best move %s is not legal: %v", depth, fen, reduced.BestMove, err)
			}
		}
	}
}

func TestLateMoveReductionKeepsMate(t *testing.T) {
	position := mustFEN(t, "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 0 1")

	result := position.Search(5)
	moves, ok := ScoreToMate(result.Score)
	if !ok || moves != 1 {
		t.Fatalf("score = %d (%d moves, %v), want mate in 1", result.Score, moves, ok)
	}
	if result.BestMove.String() != "a1a8" {
		t.Fatalf("best move = %s, want a1a8", result.BestMove)
	}
}

func TestLateMoveReductionReducesNodes(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	limits := SearchLimits{MaxDepth: 7}
	plain := position.searchWithOptions(context.Background(), limits, nil, searchOptions{pvs: true})
	reduced := position.searchWithOptions(context.Background(), limits, nil, searchOptions{pvs: true, lmr: true})

	if reduced.Nodes >= plain.Nodes {
		t.Fatalf("LMR visited %d nodes, plain %d", reduced.Nodes, plain.Nodes)
	}
	if reduced.Score != plain.Score || reduced.BestMove != plain.BestMove {
		t.Fatalf("LMR changed the result: %s/%d vs %s/%d",
			reduced.BestMove, reduced.Score, plain.BestMove, plain.Score)
	}
}
