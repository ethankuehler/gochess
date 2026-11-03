package chess

import (
	"context"
	"testing"
)

func TestStaticExchangeEvaluation(t *testing.T) {
	cases := []struct {
		name string
		fen  string
		move string
		want int
	}{
		{"free queen", freeQueenFEN, "d2d5", 900},
		{"queen takes defended pawn", defendedPawnFEN, "d1d5", -800},
		{"pawn recaptures rook", "4k3/8/2p5/3p4/8/3R4/8/4K3 w - - 0 1", "d3d5", -400},
		{"x-ray recapture", "k7/8/8/4p3/3r4/2B5/1Q6/K7 w - - 0 1", "c3d4", 270},
		{"en passant", enPassantQSFEN, "e5d6", 100},
		{"defended en passant", "3r4/8/8/3pP3/8/8/8/4K2k w - d6 0 1", "e5d6", 0},
		{"quiet queen promotion", promotionQSFEN, "e7e8q", 800},
		{"quiet knight promotion", promotionQSFEN, "e7e8n", 220},
		{"queen promotion capture", promotionCaptureFEN, "b7a8q", 1300},
		{"knight promotion capture", promotionCaptureFEN, "b7a8n", 720},
		{"back-rank pawn defends", "rn2k3/2P5/8/8/1Q6/8/8/4K3 w - - 0 1", "b4b8", 320},
		{"back-rank pawn recaptures rook", "rR6/2P5/8/8/8/8/8/4K2k b - - 0 1", "a8b8", -800},
		{"black promotion recapture", "4k3/8/8/8/8/8/Rp6/r3K3 w - - 0 1", "a2a1", -800},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			position := mustFEN(t, tc.fen)
			move := legalMoveFor(t, position, tc.move)
			if got := position.see(move); got != tc.want {
				t.Fatalf("SEE(%s) = %d, want %d", tc.move, got, tc.want)
			}
		})
	}
}

func TestQSearchSEEOrderingAndPruning(t *testing.T) {
	position := mustFEN(t, "4k3/8/1p6/r7/3p4/8/8/Q2RK3 w - - 0 1")
	losing := legalMoveFor(t, position, "a1a5")
	winning := legalMoveFor(t, position, "d1d4")
	if got := position.see(losing); got >= 0 {
		t.Fatalf("losing capture SEE = %d, want negative", got)
	}
	if got := position.see(winning); got <= 0 {
		t.Fatalf("winning capture SEE = %d, want positive", got)
	}

	base := append([]Move(nil), position.tacticalMoves(nil)...)
	mvvLVAOrdered := (&searcher{}).orderQSMoves(position, append([]Move(nil), base...), false)
	if len(mvvLVAOrdered) == 0 || mvvLVAOrdered[0] != losing {
		t.Fatalf("MVV-LVA first move = %v, want losing high-value capture %s", mvvLVAOrdered, losing)
	}

	seeOrdered := (&searcher{opts: searchOptions{see: true}}).orderQSMoves(position, append([]Move(nil), base...), false)
	if len(seeOrdered) == 0 || seeOrdered[0] != winning {
		t.Fatalf("SEE first move = %v, want %s", seeOrdered, winning)
	}

	pruned := (&searcher{opts: searchOptions{see: true, seePruning: true}}).orderQSMoves(position, append([]Move(nil), base...), true)
	if _, ok := findMove(pruned, losing.String()); ok {
		t.Errorf("negative-SEE capture %s was not pruned: %v", losing, pruned)
	}
	if _, ok := findMove(pruned, winning.String()); !ok {
		t.Errorf("positive-SEE capture %s was pruned: %v", winning, pruned)
	}
}

func TestQSearchSEEPruningKeepsPromotions(t *testing.T) {
	promotion := mustFEN(t, promotionCaptureFEN)
	moves := promotion.tacticalMoves(nil)
	got := (&searcher{opts: searchOptions{see: true, seePruning: true}}).orderQSMoves(promotion, moves, true)
	if len(got) != len(moves) {
		t.Fatalf("pruning removed a promotion: got %v, want %v", got, moves)
	}
}

// A pawn on the 7th rank defends a back-rank square and promotes when it
// recaptures, so SEE must value that exchange correctly in both directions.
func TestQSearchSEEBackRankPawnDefenders(t *testing.T) {
	s := &searcher{opts: searchOptions{see: true, seePruning: true}}

	winning := mustFEN(t, "rn2k3/2P5/8/8/1Q6/8/8/4K3 w - - 0 1")
	queen := legalMoveFor(t, winning, "b4b8")
	if got := winning.see(queen); got != 320 {
		t.Fatalf("SEE(Qxb8) = %d, want 320 (c7 guards b8)", got)
	}
	kept := s.orderQSMoves(winning, winning.tacticalMoves(nil), true)
	if _, ok := findMove(kept, queen.String()); !ok {
		t.Errorf("winning back-rank capture %s was pruned: %v", queen, kept)
	}

	losing := mustFEN(t, "rR6/2P5/8/8/8/8/8/4K2k b - - 0 1")
	rook := legalMoveFor(t, losing, "a8b8")
	if got := losing.see(rook); got != -800 {
		t.Fatalf("SEE(Rxb8) = %d, want -800 (c7 recaptures with promotion)", got)
	}
	pruned := s.orderQSMoves(losing, losing.tacticalMoves(nil), true)
	if _, ok := findMove(pruned, rook.String()); ok {
		t.Errorf("losing back-rank capture %s was not pruned: %v", rook, pruned)
	}
}

func TestQSearchSEEPruningReducesNodes(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	limits := SearchLimits{MaxDepth: 6}
	ordered := position.searchWithOptions(context.Background(), limits, nil, searchOptions{pvs: true, see: true})
	pruned := position.searchWithOptions(context.Background(), limits, nil, searchOptions{pvs: true, see: true, seePruning: true})

	if pruned.Nodes >= ordered.Nodes {
		t.Fatalf("SEE pruning visited %d nodes, ordering only %d", pruned.Nodes, ordered.Nodes)
	}
	if _, err := position.ApplyMove(pruned.BestMove); err != nil {
		t.Fatalf("best move %s is not legal: %v", pruned.BestMove, err)
	}
}
