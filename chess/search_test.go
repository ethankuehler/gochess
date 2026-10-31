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

	// Quiescence sees Kxe3 after Qxe3+, so even the depth-1 search must avoid
	// the poisoned pawn capture.
	shallow := position.Search(1)
	if got := shallow.BestMove.String(); got == "e1e3" {
		t.Fatalf("depth 1 plays the poisoned pawn capture e1e3")
	}

	deep := position.Search(2)
	if deep.BestMove.String() == "e1e3" {
		t.Fatalf("depth 2 still plays the poisoned pawn capture e1e3")
	}
}

func TestSearchResolvesCaptureChainAtLeaf(t *testing.T) {
	// Rxd5 loses a rook to cxd5, a recapture one ply beyond a depth-1 horizon.
	position := mustFEN(t, "4k3/8/2p5/3p4/8/3R4/8/4K3 w - - 0 1")

	result := position.Search(1)
	if !result.HasMove {
		t.Fatal("search returned no move")
	}
	if got := result.BestMove.String(); got == "d3d5" {
		t.Fatalf("depth 1 played the losing capture d3d5")
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

func TestSearchColourSymmetry(t *testing.T) {
	fixtures := []string{
		NewStartingPosition().FEN(),
		poisonedPawnFEN,
		freeQueenFEN,
		"4k3/8/8/3p4/8/2P5/8/4K3 w - - 0 1",
	}

	for _, fen := range fixtures {
		original := mustFEN(t, fen)
		mirrored := mustFEN(t, mirrorFEN(t, fen))
		for depth := 1; depth <= 2; depth++ {
			got := original.Search(depth).Score
			want := mirrored.Search(depth).Score
			if got != want {
				t.Errorf("depth %d on %q: original = %d, mirrored = %d (mirrored FEN %q)",
					depth, fen, got, want, mirrored.FEN())
			}
		}
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
	position := mustFEN(t, kiwipeteFEN)
	moves := position.LegalMoves()

	s := &searcher{}
	ordered := s.orderMoves(position, append([]Move(nil), moves...), Move{}, 4)
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

func TestOrderMovesMVVLVA(t *testing.T) {
	// d4xe5 is PxQ, e2xe5 is RxQ, and d4xc5 is PxP, so MVV-LVA must order them
	// exactly that way regardless of generation order.
	position := mustFEN(t, "4k3/8/8/2p1q3/3P4/8/4R3/4K3 w - - 0 1")
	moves := position.LegalMoves()

	s := &searcher{}
	ordered := s.orderMoves(position, append([]Move(nil), moves...), Move{}, 0)
	index := make(map[string]int, len(ordered))
	for i, m := range ordered {
		index[m.String()] = i
	}

	pawnTakesQueen, ok1 := index["d4e5"]
	rookTakesQueen, ok2 := index["e2e5"]
	pawnTakesPawn, ok3 := index["d4c5"]
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("missing captures in ordering: %v", index)
	}
	if !(pawnTakesQueen < rookTakesQueen && rookTakesQueen < pawnTakesPawn) {
		t.Fatalf("MVV-LVA order is %d, %d, %d; want increasing", pawnTakesQueen, rookTakesQueen, pawnTakesPawn)
	}
}

func TestOrderMovesHashKillerHistory(t *testing.T) {
	position := mustFEN(t, "4k3/8/8/2p1q3/3P4/8/4R3/4K3 w - - 0 1")
	moves := position.LegalMoves()

	hash := legalMoveFor(t, position, "e1d1")
	killer := legalMoveFor(t, position, "e1f1")
	scored := legalMoveFor(t, position, "e2e3")
	plain := legalMoveFor(t, position, "e2e4")

	s := &searcher{}
	s.killers[3][0] = killer
	s.history[White][scored.From()][scored.To()] = 4000

	ordered := s.orderMoves(position, append([]Move(nil), moves...), hash, 3)
	index := make(map[string]int, len(ordered))
	for i, m := range ordered {
		index[m.String()] = i
	}
	if index[hash.String()] != 0 {
		t.Fatalf("hash move is at %d, want 0", index[hash.String()])
	}
	if index[killer.String()] > index[scored.String()] {
		t.Fatalf("killer %s ordered after history move %s", killer, scored)
	}
	if index[scored.String()] > index[plain.String()] {
		t.Fatalf("history move %s ordered after quiet move %s", scored, plain)
	}
}

func TestRecordQuietCutoff(t *testing.T) {
	s := &searcher{}
	first, err := ParseUCIMove("a2a3")
	if err != nil {
		t.Fatalf("ParseUCIMove(a2a3): %v", err)
	}
	second, err := ParseUCIMove("b2b3")
	if err != nil {
		t.Fatalf("ParseUCIMove(b2b3): %v", err)
	}

	s.recordQuietCutoff(White, *first, 3, 4)
	s.recordQuietCutoff(White, *second, 3, 4)
	if s.killers[4][0] != *second || s.killers[4][1] != *first {
		t.Fatalf("killers = %v, want [%s %s]", s.killers[4], second, first)
	}
	s.recordQuietCutoff(White, *first, 3, 4)
	if s.killers[4][0] != *first || s.killers[4][1] != *second {
		t.Fatalf("killers after repeat = %v, want [%s %s]", s.killers[4], first, second)
	}

	from, to := first.From(), first.To()
	if got := s.history[White][from][to]; got != 18 {
		t.Fatalf("history = %d, want 18", got)
	}
	s.history[White][from][to] = historyMax - 1
	s.recordQuietCutoff(White, *first, 2, 4)
	if got := s.history[White][from][to]; got > historyMax/2+1 {
		t.Fatalf("history = %d after saturation, want it halved", got)
	}
}

func TestAlphaBetaMatchesMinimax(t *testing.T) {
	// The reference is full-width and the reference qsearch has no pruning, so
	// dense positions are limited to shallow depths. The value here is score
	// equivalence between alpha-beta and plain minimax, not coverage volume;
	// dense middlegame scores are covered by the direct qsearch tests and by
	// legality, determinism, and symmetry checks on the production search.
	fixtures := []struct {
		name     string
		fen      string
		maxDepth int
	}{
		{"start", NewStartingPosition().FEN(), 2},
		{"poisoned-pawn", poisonedPawnFEN, 3},
		{"queen-vs-king", "8/8/8/4k3/8/8/3Q4/4K3 w - - 0 1", 3},
		{"free-queen", freeQueenFEN, 3},
		{"pawn-capture", "4k3/8/8/3p4/8/2P5/8/4K3 w - - 0 1", 3},
		{"defended-pawn", defendedPawnFEN, 3},
		{"en-passant", enPassantQSFEN, 3},
		{"check-evasion", inCheckRookFEN, 3},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			position := mustFEN(t, tc.fen)
			for depth := 1; depth <= tc.maxDepth; depth++ {
				want := referenceMinimax(position, depth, 0)
				got := position.Search(depth).Score
				if got != want {
					t.Errorf("depth %d on %q: alpha-beta score = %d, minimax = %d", depth, tc.fen, got, want)
				}
			}
		})
	}
}

// referenceMinimax is a full-width minimax with the same evaluation, terminal,
// draw, and repetition rules as Search. It exists only to validate alpha-beta
// pruning and the leaf integration in tests. Depth zero delegates to the
// full-width referenceQuiesce in qsearch_test.go; both share one key path so
// repetition is detected in the same places as production. It has no
// transposition table and no pruning.
func referenceMinimax(b *Position, depth, ply int) int {
	return referenceMinimaxAt(&searcher{}, b, depth, ply)
}

func referenceMinimaxAt(s *searcher, b *Position, depth, ply int) int {
	if depth == 0 {
		return referenceQuiesceAt(s, b, ply)
	}
	if index := s.pathBase + ply; index < len(s.keys) {
		s.keys[index] = b.key
	}

	var buf [256]Move
	moves := b.legalMoves(buf[:0])
	if len(moves) == 0 {
		if b.InCheck() {
			return -mateScore + ply
		}
		return 0
	}
	if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() ||
		s.repetitionDraw(b.key, int(b.halfmoveClock), ply) {
		return 0
	}

	best := -infinity
	for _, m := range moves {
		child := b.makeMove(m)
		score := 0
		if !s.repeatsEarlier(child.key, int(child.halfmoveClock), ply) {
			score = -referenceMinimaxAt(s, &child, depth-1, ply+1)
		}
		if score > best {
			best = score
		}
	}
	return best
}
