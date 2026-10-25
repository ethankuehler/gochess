package chess

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

func referenceLegalMoves(b *Position, moves []Move) []Move {
	pseudo := b.pseudoLegalMoves(moves)
	us := b.sideToMove()
	them := us.Opposite()

	legal := pseudo[:0]
	for _, m := range pseudo {
		next := b.makeMoveForLegality(m)
		if !next.isSquareAttacked(next.kingSquare(us), them) {
			legal = append(legal, m)
		}
	}
	return legal
}

func requireMovesMatchReference(t *testing.T, b *Position, generate func() []Move) {
	t.Helper()
	before := *b
	got := generate()
	if *b != before {
		t.Fatalf("%s: generation mutated the position", b.FEN())
	}
	want := referenceLegalMoves(b, nil)
	if *b != before {
		t.Fatalf("%s: reference generation mutated the position", b.FEN())
	}
	if !slices.Equal(got, want) {
		first := 0
		for first < len(got) && first < len(want) && got[first] == want[first] {
			first++
		}
		t.Fatalf("%s: move sequences differ at index %d\ngot:  %v\nwant: %v", b.FEN(), first, got, want)
	}
	for i, m := range got {
		gotNext := b.makeMove(m)
		wantNext := b.makeMove(want[i])
		if gotNext.FEN() != wantNext.FEN() || gotNext.key != wantNext.key {
			t.Fatalf("%s: applied move %d (%s) differs", b.FEN(), i, m)
		}
	}
}

func requireLegalMovesMatchReference(t *testing.T, b *Position) {
	t.Helper()
	requireMovesMatchReference(t, b, func() []Move { return b.LegalMoves() })
	requireMovesMatchReference(t, b, func() []Move { return b.strictlyLegalMoves(nil) })
}

func TestLegalMovesMatchReferencePerftCorpus(t *testing.T) {
	seen := make(map[string]bool)
	for i, tc := range loadPerftCases(t) {
		if seen[tc.fen] {
			continue
		}
		seen[tc.fen] = true
		t.Run(fmt.Sprintf("case_%03d", i), func(t *testing.T) {
			requireLegalMovesMatchReference(t, mustFEN(t, tc.fen))
		})
	}
}

func TestLegalMovesMatchReferenceRandomGames(t *testing.T) {
	rng := rand.New(rand.NewPCG(0x4d595df4d0f33173, 0x14057b7ef767814f))
	for game := 0; game < 6; game++ {
		b := NewStartingPosition()
		for ply := 0; ply < 120; ply++ {
			requireLegalMovesMatchReference(t, b)
			moves := referenceLegalMoves(b, nil)
			if len(moves) == 0 {
				break
			}
			next := b.makeMove(moves[rng.IntN(len(moves))])
			b = &next
		}
	}
}
