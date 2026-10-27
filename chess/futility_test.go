package chess

import (
	"context"
	"testing"
)

func TestFutilityAllowed(t *testing.T) {
	s := &searcher{opts: searchOptions{futility: true}}
	cases := []struct {
		name    string
		depth   int
		pv      bool
		inCheck bool
		alpha   int
		beta    int
		want    bool
	}{
		{"frontier scout", 2, false, false, 0, 1, true},
		{"depth 1", 1, false, false, 0, 1, true},
		{"too deep", 3, false, false, 0, 1, false},
		{"pv node", 2, true, false, 0, 1, false},
		{"in check", 2, false, true, 0, 1, false},
		{"mate window", 2, false, false, mateScore - 10, mateScore - 5, false},
	}
	for _, tc := range cases {
		if got := s.futilityAllowed(tc.depth, tc.pv, tc.inCheck, tc.alpha, tc.beta); got != tc.want {
			t.Errorf("%s: allowed = %v, want %v", tc.name, got, tc.want)
		}
	}
	s.opts.futility = false
	if s.futilityAllowed(2, false, false, 0, 1) {
		t.Fatal("futility allowed with the feature disabled")
	}
}

func TestFutilityPrunes(t *testing.T) {
	position := mustFEN(t, freeQueenFEN)
	quiet := legalMoveFor(t, position, "e1d1")
	capture := legalMoveFor(t, position, "d2d5")
	promoting := mustFEN(t, "4k3/6P1/8/8/8/8/8/4K3 w - - 0 1")
	promotion := legalMoveFor(t, promoting, "g7g8q")

	s := &searcher{opts: searchOptions{futility: true}}
	cases := []struct {
		name     string
		move     Move
		hash     Move
		depth    int
		eval     int
		alpha    int
		haveEval bool
		best     int
		want     bool
	}{
		{"futile", quiet, Move{}, 2, 0, 300, true, 0, true},
		{"not futile", quiet, Move{}, 2, 200, 300, true, 0, false},
		{"no eval", quiet, Move{}, 2, 0, 300, false, 0, false},
		{"first move", quiet, Move{}, 2, 0, 300, true, -infinity, false},
		{"hash move", quiet, quiet, 2, 0, 300, true, 0, false},
		{"capture", capture, Move{}, 2, 0, 300, true, 0, false},
		{"promotion", promotion, Move{}, 2, 0, 300, true, 0, false},
	}
	for _, tc := range cases {
		if got := s.futilityPrunes(tc.move, tc.hash, tc.depth, tc.eval, tc.alpha, tc.haveEval, tc.best); got != tc.want {
			t.Errorf("%s: pruned = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestFutilityReducesNodes(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	limits := SearchLimits{MaxDepth: 6}
	plain := position.searchWithOptions(context.Background(), limits, nil, searchOptions{pvs: true, lmr: true})
	pruned := position.searchWithOptions(context.Background(), limits, nil, searchOptions{pvs: true, lmr: true, futility: true})

	if pruned.Nodes >= plain.Nodes {
		t.Fatalf("futility visited %d nodes, plain %d", pruned.Nodes, plain.Nodes)
	}
	if drop := plain.Score - pruned.Score; drop > 100 {
		t.Fatalf("futility score %d, plain %d", pruned.Score, plain.Score)
	}
	if _, err := position.ApplyMove(pruned.BestMove); err != nil {
		t.Fatalf("best move %s is not legal: %v", pruned.BestMove, err)
	}
}
