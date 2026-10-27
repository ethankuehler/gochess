package chess

import (
	"context"
	"math/rand/v2"
	"testing"
)

func TestTranspositionTableStoreProbe(t *testing.T) {
	tt := NewTranspositionTable(1)
	tt.newSearch()
	from, _ := ParseSquare("e2")
	to, _ := ParseSquare("e4")
	move := newMove(from, to, doublePushFlag)
	tt.store(0x1234, 4, 42, ttLower, move)

	entry, ok := tt.probe(0x1234)
	if !ok {
		t.Fatal("stored key was not found")
	}
	if entry.key != 0x1234 || entry.score != 42 || entry.depth != 4 || entry.bound != ttLower || entry.move != move {
		t.Fatalf("entry = %+v, want key 0x1234 score 42 depth 4 lower move %s", entry, move)
	}
	if _, ok := tt.probe(0x1235); ok {
		t.Fatal("different key matched")
	}
}

func TestTranspositionTableClear(t *testing.T) {
	tt := NewTranspositionTable(1)
	tt.newSearch()
	tt.store(0x42, 3, 7, ttExact, Move{})
	if _, ok := tt.probe(0x42); !ok {
		t.Fatal("stored key was not found")
	}

	tt.Clear()
	if _, ok := tt.probe(0x42); ok {
		t.Fatal("Clear left an entry behind")
	}

	tt.newSearch()
	tt.store(0x42, 3, 7, ttExact, Move{})
	entry, ok := tt.probe(0x42)
	if !ok {
		t.Fatal("store after Clear failed")
	}
	if entry.age != 1 {
		t.Fatalf("age after Clear and one newSearch = %d, want 1", entry.age)
	}
}

func TestTranspositionTableSizing(t *testing.T) {
	for _, mb := range []int{MinHashMB, 4, DefaultHashMB} {
		tt := NewTranspositionTable(mb)
		entries := tt.Entries()
		if entries < 1 || entries&(entries-1) != 0 {
			t.Fatalf("NewTranspositionTable(%d) has %d entries, want a positive power of two", mb, entries)
		}
		if tt.mask != uint64(entries-1) {
			t.Fatalf("mask = %d, entries = %d", tt.mask, entries)
		}
	}
	if got := ttEntryCount(0); got != ttEntryCount(MinHashMB) {
		t.Errorf("size 0 gave %d entries, want the minimum %d", got, ttEntryCount(MinHashMB))
	}
	if got := ttEntryCount(MaxHashMB + 1); got != ttEntryCount(MaxHashMB) {
		t.Errorf("oversized table has %d entries, want the maximum %d", got, ttEntryCount(MaxHashMB))
	}
}

func TestTranspositionTableAgeWrapClears(t *testing.T) {
	tt := NewTranspositionTable(1)
	tt.newSearch()
	tt.store(0x77, 3, 7, ttExact, Move{})
	for i := 0; i < 255; i++ {
		tt.newSearch()
	}
	if tt.age != 0 {
		t.Fatalf("age = %d after 256 generations, want 0", tt.age)
	}
	if _, ok := tt.probe(0x77); ok {
		t.Fatal("entry survived the generation wrap")
	}
}

func TestTranspositionTableReplacement(t *testing.T) {
	tt := NewTranspositionTable(1)
	entries := uint64(tt.Entries())
	tt.newSearch()

	// Same key: a shallower result must not displace a deeper one.
	tt.store(0x10, 5, 50, ttExact, Move{})
	tt.store(0x10, 3, 30, ttExact, Move{})
	if entry, _ := tt.probe(0x10); entry.depth != 5 || entry.score != 50 {
		t.Fatalf("shallower store replaced a deeper entry: %+v", entry)
	}
	// Equal depth replaces.
	tt.store(0x10, 5, 51, ttUpper, Move{})
	if entry, _ := tt.probe(0x10); entry.score != 51 || entry.bound != ttUpper {
		t.Fatalf("equal-depth store did not replace: %+v", entry)
	}

	// Index collision: same slot, different key. The deeper current-generation
	// entry wins; a stale-generation entry is replaced even when shallower.
	collision := 0x10 + entries
	tt.store(0x10, 6, 60, ttExact, Move{})
	tt.store(collision, 4, 40, ttExact, Move{})
	if entry, _ := tt.probe(0x10); entry.key != 0x10 || entry.depth != 6 {
		t.Fatalf("shallow collision displaced a deeper entry: %+v", entry)
	}
	tt.store(collision, 7, 70, ttExact, Move{})
	if entry, _ := tt.probe(collision); entry.key != collision || entry.depth != 7 {
		t.Fatalf("deeper collision did not replace: %+v", entry)
	}

	tt.newSearch()
	stale := collision + entries
	tt.store(stale, 1, 10, ttExact, Move{})
	if entry, _ := tt.probe(stale); entry.key != stale || entry.depth != 1 || entry.score != 10 {
		t.Fatalf("stale-generation entry survived: %+v", entry)
	}
}

func TestMateScoreTTConversion(t *testing.T) {
	cases := []struct {
		name       string
		score, ply int
		probePly   int
		want       int
	}{
		{"winning mate seen deeper", MateScore - 3, 5, 8, MateScore - 6},
		{"winning mate same ply", MateScore - 3, 5, 5, MateScore - 3},
		{"losing mate seen deeper", -MateScore + 5, 5, 8, -MateScore + 8},
		{"losing mate same ply", -MateScore + 5, 5, 5, -MateScore + 5},
		{"ordinary score", 123, 5, 8, 123},
		{"ordinary negative score", -456, 9, 2, -456},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stored := scoreToTT(tc.score, tc.ply)
			if got := scoreFromTT(stored, tc.probePly); got != tc.want {
				t.Fatalf("scoreFromTT(scoreToTT(%d, %d), %d) = %d, want %d",
					tc.score, tc.ply, tc.probePly, got, tc.want)
			}
		})
	}
}

func TestSearchWithTTMatchesWithout(t *testing.T) {
	fixtures := []string{
		NewStartingPosition().FEN(),
		kiwipeteFEN,
		poisonedPawnFEN,
		"8/8/8/4k3/8/8/3Q4/4K3 w - - 0 1",
	}
	// LMR and futility are heuristic, so the table (which changes ordering and
	// therefore which moves are reduced) may legitimately perturb a score.
	// Compare the sound feature set here; the tactical tests cover the rest.
	sound := searchOptions{pvs: true, aspiration: true}

	var totalHits uint64
	for _, fen := range fixtures {
		position := mustFEN(t, fen)
		for depth := 3; depth <= 4; depth++ {
			plain := position.searchWithOptions(context.Background(), SearchLimits{MaxDepth: depth}, nil, sound)
			tt := NewTranspositionTable(DefaultHashMB)
			cached := position.searchWithOptions(context.Background(), SearchLimits{MaxDepth: depth, TT: tt}, nil, sound)

			if cached.Score != plain.Score {
				t.Errorf("depth %d on %q: TT score = %d, plain = %d", depth, fen, cached.Score, plain.Score)
			}
			if cached.HasMove != plain.HasMove {
				t.Errorf("depth %d on %q: TT HasMove = %v, plain = %v", depth, fen, cached.HasMove, plain.HasMove)
			}
			// Only the table supplies a hash move, so the two searches order
			// moves differently and can trade a few nodes either way. Require
			// no meaningful regression rather than a strict win.
			if cached.Nodes > plain.Nodes+plain.Nodes/20+16 {
				t.Errorf("depth %d on %q: TT visited %d nodes, plain %d", depth, fen, cached.Nodes, plain.Nodes)
			}
			if cached.TT.Probes == 0 {
				t.Errorf("depth %d on %q: no table probes: %+v", depth, fen, cached.TT)
			}
			if cached.TT.Cutoffs > cached.TT.Hits {
				t.Errorf("depth %d on %q: %d cutoffs from %d hits", depth, fen, cached.TT.Cutoffs, cached.TT.Hits)
			}
			if len(cached.PV) == 0 || cached.PV[0] != cached.BestMove {
				t.Errorf("depth %d on %q: PV = %v, best move = %s", depth, fen, cached.PV, cached.BestMove)
			}
			assertPVLegal(t, position, cached.PV)
			totalHits += cached.TT.Hits
		}
	}
	if totalHits == 0 {
		t.Fatal("no fixture produced a transposition-table hit")
	}
}

func TestSearchTTReuseReducesNodes(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	tt := NewTranspositionTable(DefaultHashMB)

	first := position.SearchWithLimits(context.Background(), SearchLimits{MaxDepth: 4, TT: tt}, nil)
	second := position.SearchWithLimits(context.Background(), SearchLimits{MaxDepth: 4, TT: tt}, nil)

	if first.Score != second.Score {
		t.Fatalf("scores differ across reuse: first %d, second %d", first.Score, second.Score)
	}
	if second.TT.Hits == 0 {
		t.Fatal("second search got no table hits")
	}
	if second.Nodes >= first.Nodes {
		t.Fatalf("second search visited %d nodes, first %d; reuse did not help", second.Nodes, first.Nodes)
	}
}

func TestSearchTTPreservesMateScores(t *testing.T) {
	mateInOne := mustFEN(t, "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - - 0 1")
	tt := NewTranspositionTable(DefaultHashMB)
	result := mateInOne.SearchWithLimits(context.Background(), SearchLimits{MaxDepth: 4, TT: tt}, nil)
	if result.Score != MateScore-1 {
		t.Fatalf("mate-in-one score = %d, want %d", result.Score, MateScore-1)
	}
	if got := result.BestMove.String(); got != "a1a8" {
		t.Fatalf("best move = %s, want a1a8", got)
	}

	// A mate score stored at one ply must keep the same distance when it is
	// probed at another ply.
	tt.newSearch()
	stored := scoreToTT(MateScore-5, 3)
	tt.store(0x99, 4, stored, ttExact, Move{})
	entry, ok := tt.probe(0x99)
	if !ok {
		t.Fatal("mate entry missing")
	}
	if got := scoreFromTT(entry.score, 7); got != MateScore-9 {
		t.Fatalf("probed mate score = %d, want %d", got, MateScore-9)
	}
}

func TestSearchTTRespectsRepetitionAndFiftyMoveDraws(t *testing.T) {
	position := mustFEN(t, "4k3/8/8/3q4/8/8/3R4/4K3 w - - 2 1")
	quiet := legalMoveFor(t, position, "d2d3")
	child, _, err := position.ApplyUCIMove("d2d3")
	if err != nil {
		t.Fatalf("ApplyUCIMove(d2d3): %v", err)
	}

	tt := NewTranspositionTable(DefaultHashMB)
	repeat := position.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth:    4,
		RootMoves:   []Move{quiet},
		GameHistory: []PositionKey{child.Key(), child.Key()},
		TT:          tt,
	}, nil)
	if repeat.Score != 0 {
		t.Fatalf("repetition score = %d, want 0", repeat.Score)
	}

	fifty := mustFEN(t, "4k3/8/8/3q4/8/8/3R4/4K3 w - - 99 1")
	quiet = legalMoveFor(t, fifty, "d2d3")
	fiftyResult := fifty.SearchWithLimits(context.Background(), SearchLimits{
		MaxDepth:  4,
		RootMoves: []Move{quiet},
		TT:        tt,
	}, nil)
	if fiftyResult.Score != 0 {
		t.Fatalf("fifty-move score = %d, want 0", fiftyResult.Score)
	}
}

func TestSearchWithoutTTReportsNoTableActivity(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	result := position.SearchWithLimits(context.Background(), SearchLimits{MaxDepth: 3}, nil)
	if result.TT != (TTStats{}) {
		t.Fatalf("search without a table reported %+v", result.TT)
	}
}

// TestSearchTTMatchesWithoutAcrossRandomGame compares table and no-table
// searches along a deterministic random game. Real games exercise castling,
// en passant, promotions, checks, and repetition history that the small fixed
// fixtures do not.
func TestSearchTTMatchesWithoutAcrossRandomGame(t *testing.T) {
	position := NewStartingPosition()
	history := make([]PositionKey, 0, 60)
	rng := rand.New(rand.NewPCG(0x5eed, 0x1234))

	for ply := 0; ply < 40; ply++ {
		moves := position.LegalMoves()
		if len(moves) == 0 {
			break
		}

		sound := searchOptions{pvs: true, aspiration: true}
		plain := position.searchWithOptions(context.Background(), SearchLimits{
			MaxDepth:    4,
			GameHistory: history,
		}, nil, sound)
		tt := NewTranspositionTable(DefaultHashMB)
		cached := position.searchWithOptions(context.Background(), SearchLimits{
			MaxDepth:    4,
			GameHistory: history,
			TT:          tt,
		}, nil, sound)
		if cached.Score != plain.Score {
			t.Fatalf("ply %d (%s): TT score = %d, plain = %d; history has %d keys",
				ply, position.FEN(), cached.Score, plain.Score, len(history))
		}

		move := moves[rng.IntN(len(moves))]
		history = append(history, position.Key())
		next, err := position.ApplyMove(move)
		if err != nil {
			t.Fatalf("ply %d: apply %s: %v", ply, move, err)
		}
		position = next
	}
}
