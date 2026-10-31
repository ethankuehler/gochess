package chess

import (
	"context"
	"testing"
	"time"
)

const (
	defendedPawnFEN      = "8/8/8/3p4/4k3/8/8/3QK3 w - - 0 1"
	enPassantQSFEN       = "8/8/8/3pP3/8/8/8/4K2k w - d6 0 1"
	promotionQSFEN       = "8/4P3/8/8/8/8/k7/4K3 w - - 0 1"
	promotionCaptureFEN  = "r7/1P6/8/8/8/8/8/k3K3 w - - 0 1"
	mateAtHorizonFEN     = "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1"
	stalemateQSFEN       = "7k/5Q2/6K1/8/8/8/8/8 b - - 0 1"
	inCheckRookFEN       = "4r1k1/8/8/8/8/8/8/4K3 w - - 0 1"
	inCheckQueenFEN      = "4q1k1/8/8/8/8/8/8/4K3 w - - 0 1"
	inCheckCaptureFEN    = "4k3/8/8/8/8/8/4r3/4K3 w - - 0 1"
	inCheckRankRookFEN   = "4k3/8/8/8/8/8/8/4K2r w - - 0 1"
	repetitionQSFEN      = "4k3/8/8/3q4/8/8/3R4/4K3 w - - 4 1"
	insufficientQSFEN    = "4k3/8/8/8/8/8/8/4K3 w - - 0 1"
	fiftyMoveQSFEN       = "4k3/8/8/8/8/8/8/R3K3 w - - 100 1"
	fiftyMoveMateQSFEN   = "7k/6Q1/6K1/8/8/8/8/8 b - - 100 1"
	blackWinningQSFEN    = "4k3/8/8/3q4/8/8/3R4/4K3 b - - 0 1"
	castlingQSFEN        = "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1"
	freePawnCaptureQSFEN = "4k3/8/8/8/4p3/3Q4/8/4K3 w - - 0 1"
)

func quiesceAt(t *testing.T, position *Position, ply int) int {
	t.Helper()
	s := &searcher{}
	return s.quiesce(*position, -infinity, infinity, ply)
}

func TestQuiesceCapturesFreePiece(t *testing.T) {
	position := mustFEN(t, freeQueenFEN)
	standPat := position.evaluateForSideToMove()

	got := quiesceAt(t, position, 0)
	if got-standPat < 700 {
		t.Fatalf("quiesce score %d only improves on stand-pat %d by %d; want at least 700", got, standPat, got-standPat)
	}
}

func TestQuiesceSeesRecapture(t *testing.T) {
	position := mustFEN(t, defendedPawnFEN)
	standPat := position.evaluateForSideToMove()

	if got := quiesceAt(t, position, 0); got != standPat {
		t.Fatalf("quiesce score = %d, want stand-pat %d", got, standPat)
	}
}

func TestQuiesceEnPassant(t *testing.T) {
	position := mustFEN(t, enPassantQSFEN)
	standPat := position.evaluateForSideToMove()

	got := quiesceAt(t, position, 0)
	if got-standPat < 100 {
		t.Fatalf("quiesce score %d only improves on stand-pat %d by %d; want at least 100", got, standPat, got-standPat)
	}
}

func TestQuiescePromotion(t *testing.T) {
	position := mustFEN(t, promotionQSFEN)
	standPat := position.evaluateForSideToMove()

	got := quiesceAt(t, position, 0)
	if got-standPat < 700 {
		t.Fatalf("quiesce score %d only improves on stand-pat %d by %d; want at least 700", got, standPat, got-standPat)
	}
}

func TestQuiescePromotionCapture(t *testing.T) {
	position := mustFEN(t, promotionCaptureFEN)
	standPat := position.evaluateForSideToMove()

	got := quiesceAt(t, position, 0)
	if got-standPat < 1000 {
		t.Fatalf("quiesce score %d only improves on stand-pat %d by %d; want at least 1000", got, standPat, got-standPat)
	}
}

func TestQuiesceSearchesOnlyForcingMoves(t *testing.T) {
	position := mustFEN(t, NewStartingPosition().FEN())
	got := quiesceAt(t, position, 0)
	if want := position.evaluateForSideToMove(); got != want {
		t.Fatalf("quiesce score = %d, want stand-pat %d", got, want)
	}
}

func TestOrderQSMoves(t *testing.T) {
	position := mustFEN(t, promotionQSFEN)
	moves := filterCapturesAndPromotions(position.LegalMoves())
	if len(moves) != 4 {
		t.Fatalf("filtered %d promotions, want 4", len(moves))
	}

	orderQSMoves(position, moves)
	if piece, ok := moves[0].Promotion(); !ok || piece != Queen {
		t.Fatalf("first ordered move = %s, want a queen promotion", moves[0])
	}
	for i := 1; i < len(moves); i++ {
		if prev, cur := qsMoveScore(position, moves[i-1]), qsMoveScore(position, moves[i]); prev < cur {
			t.Fatalf("move %d scores %d below move %d at %d", i-1, prev, i, cur)
		}
	}
}

func TestOrderQSMovesStableForQuietEvasions(t *testing.T) {
	position := mustFEN(t, inCheckRookFEN)
	moves := position.LegalMoves()
	want := append([]Move(nil), moves...)
	if len(want) < 2 {
		t.Fatalf("fixture has %d legal moves, want at least 2", len(want))
	}

	orderQSMoves(position, moves)
	for i := range want {
		if moves[i] != want[i] {
			t.Fatalf("quiet evasion order changed at %d: got %s, want %s", i, moves[i], want[i])
		}
	}
}

func TestFilterCapturesAndPromotions(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	all := position.LegalMoves()
	original := append([]Move(nil), all...)

	kept := filterCapturesAndPromotions(all)
	isForcing := func(m Move) bool {
		_, promotion := m.Promotion()
		return m.IsCapture() || promotion
	}

	want := 0
	for _, m := range original {
		if isForcing(m) {
			want++
		}
	}
	if len(kept) != want {
		t.Fatalf("kept %d forcing moves, want %d", len(kept), want)
	}

	seen := make(map[Move]int, len(kept))
	for _, m := range kept {
		if !isForcing(m) {
			t.Fatalf("filter kept non-forcing move %s", m)
		}
		seen[m]++
	}
	for _, m := range original {
		if isForcing(m) && seen[m] != 1 {
			t.Fatalf("forcing move %s appears %d times after filtering, want 1", m, seen[m])
		}
	}
}

func TestQuiesceMateAtHorizon(t *testing.T) {
	position := mustFEN(t, mateAtHorizonFEN)

	got := quiesceAt(t, position, 1)
	if want := -mateScore + 1; got != want {
		t.Fatalf("mate score = %d, want %d", got, want)
	}
}

func TestQuiesceSearchesQuietEvasions(t *testing.T) {
	fixtures := []string{
		inCheckRookFEN,
		inCheckQueenFEN,
		inCheckCaptureFEN,
		inCheckRankRookFEN,
		mateAtHorizonFEN,
	}

	for _, fen := range fixtures {
		position := mustFEN(t, fen)
		for _, ply := range []int{1, 2} {
			got := quiesceAt(t, position, ply)
			want := referenceQuiesce(position, ply)
			if got != want {
				t.Errorf("ply %d on %q: quiesce = %d, reference = %d", ply, fen, got, want)
			}
		}
	}
}

func TestQuiesceDrawLeaves(t *testing.T) {
	t.Run("fifty-move", func(t *testing.T) {
		position := mustFEN(t, fiftyMoveQSFEN)
		if got := quiesceAt(t, position, 0); got != 0 {
			t.Fatalf("fifty-move leaf = %d, want 0", got)
		}
	})

	t.Run("checkmate beats fifty-move", func(t *testing.T) {
		position := mustFEN(t, fiftyMoveMateQSFEN)
		if got, want := quiesceAt(t, position, 1), -mateScore+1; got != want {
			t.Fatalf("mate at a fifty-move leaf = %d, want %d", got, want)
		}
	})

	t.Run("insufficient material", func(t *testing.T) {
		position := mustFEN(t, insufficientQSFEN)
		if got := quiesceAt(t, position, 0); got != 0 {
			t.Fatalf("insufficient-material leaf = %d, want 0", got)
		}
	})

	t.Run("threefold repetition", func(t *testing.T) {
		position := mustFEN(t, repetitionQSFEN)
		s := &searcher{}
		s.keys[0] = position.key
		s.keys[1] = position.key
		if got := s.quiesce(*position, -infinity, infinity, 2); got != 0 {
			t.Fatalf("repetition leaf = %d, want 0", got)
		}

		control := &searcher{}
		if got := control.quiesce(*position, -infinity, infinity, 2); got == 0 {
			t.Fatal("control quiesce scored 0 without a threefold repetition")
		}
	})
}

func TestQuiesceStalemateUsesStandPat(t *testing.T) {
	position := mustFEN(t, stalemateQSFEN)
	if len(position.LegalMoves()) != 0 {
		t.Fatal("fixture is not stalemate")
	}

	got := quiesceAt(t, position, 1)
	if want := position.evaluateForSideToMove(); got != want {
		t.Fatalf("stalemate leaf = %d, want stand-pat %d", got, want)
	}
}

func TestQuiesceMaxPly(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	s := &searcher{}

	got := s.quiesce(*position, -infinity, infinity, maxPly)
	if want := position.evaluateForSideToMove(); got != want {
		t.Fatalf("max-ply score = %d, want static %d", got, want)
	}
	if s.seldepth != maxPly {
		t.Fatalf("seldepth = %d, want %d", s.seldepth, maxPly)
	}
}

func TestQuiesceColourSymmetry(t *testing.T) {
	fixtures := []string{
		NewStartingPosition().FEN(),
		kiwipeteFEN,
		poisonedPawnFEN,
		freeQueenFEN,
		enPassantQSFEN,
		promotionQSFEN,
		inCheckRookFEN,
	}

	for _, fen := range fixtures {
		original := mustFEN(t, fen)
		mirrored := mustFEN(t, mirrorFEN(t, fen))

		got := quiesceAt(t, original, 1)
		want := quiesceAt(t, mirrored, 1)
		if got != want {
			t.Errorf("on %q: original = %d, mirrored = %d (mirrored FEN %q)", fen, got, want, mirrorFEN(t, fen))
		}
	}
}

func TestQuiesceDeterministic(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	first, second := &searcher{}, &searcher{}

	a := first.quiesce(*position, -infinity, infinity, 0)
	b := second.quiesce(*position, -infinity, infinity, 0)
	if a != b || first.nodes != second.nodes {
		t.Fatalf("quiesce is not deterministic: %d/%d nodes, %d/%d score", a, b, first.nodes, second.nodes)
	}
}

func TestQuiesceDoesNotMutatePosition(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	before := position.FEN()
	beforeKey := position.Key()

	quiesceAt(t, position, 0)

	if after := position.FEN(); after != before {
		t.Fatalf("quiesce mutated the position: before %q, after %q", before, after)
	}
	if after := position.Key(); after != beforeKey {
		t.Fatalf("quiesce mutated the position key: before %d, after %d", beforeKey, after)
	}
}

func TestQuiesceMatchesReference(t *testing.T) {
	fixtures := []string{
		NewStartingPosition().FEN(),
		poisonedPawnFEN,
		freeQueenFEN,
		defendedPawnFEN,
		enPassantQSFEN,
		promotionQSFEN,
		promotionCaptureFEN,
		freePawnCaptureQSFEN,
		blackWinningQSFEN,
		castlingQSFEN,
		inCheckRookFEN,
		inCheckQueenFEN,
		inCheckCaptureFEN,
		inCheckRankRookFEN,
		mateAtHorizonFEN,
	}

	for _, fen := range fixtures {
		position := mustFEN(t, fen)
		for _, ply := range []int{0, 1, 2} {
			got := quiesceAt(t, position, ply)
			want := referenceQuiesce(position, ply)
			if got != want {
				t.Errorf("ply %d on %q: quiesce = %d, reference = %d", ply, fen, got, want)
			}
		}
	}
}

func TestSearchNodeLimitIncludesQuiescence(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)

	for _, limit := range []uint64{1, 17, 300} {
		result := position.SearchWithLimits(context.Background(), SearchLimits{
			MaxDepth: 6,
			MaxNodes: limit,
		}, nil)
		if result.Nodes > limit {
			t.Fatalf("limit %d: visited %d nodes", limit, result.Nodes)
		}
		if !result.HasMove {
			t.Fatalf("limit %d: search returned no fallback move", limit)
		}
		if _, ok := position.FindLegalMove(result.BestMove); !ok {
			t.Fatalf("limit %d: best move %s is not legal", limit, result.BestMove)
		}
	}
}

func TestSearchCancellationIncludesQuiescence(t *testing.T) {
	position := mustFEN(t, kiwipeteFEN)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(2*time.Millisecond, cancel)
	defer timer.Stop()

	result := position.SearchWithLimits(ctx, SearchLimits{MaxDepth: 30}, nil)
	if !result.HasMove {
		t.Fatal("cancelled search returned no fallback move")
	}
	if _, ok := position.FindLegalMove(result.BestMove); !ok {
		t.Fatalf("best move %s is not legal", result.BestMove)
	}
	if result.Depth >= 30 {
		t.Fatalf("search reached depth %d despite cancellation", result.Depth)
	}
	if len(result.PV) == 0 || result.PV[0] != result.BestMove {
		t.Fatalf("PV = %v, best move = %s", result.PV, result.BestMove)
	}
}

// referenceQuiesce is a full-width oracle with no pruning or table.
func referenceQuiesce(b *Position, ply int) int {
	return referenceQuiesceAt(&searcher{}, b, ply)
}

func referenceQuiesceAt(s *searcher, b *Position, ply int) int {
	if index := s.pathBase + ply; index < len(s.keys) {
		s.keys[index] = b.key
	}
	if ply >= maxPly {
		if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() {
			return 0
		}
		return b.evaluateForSideToMove()
	}

	var buf [256]Move
	if b.InCheck() {
		moves := b.legalMoves(buf[:0])
		if len(moves) == 0 {
			return -mateScore + ply
		}
		if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() ||
			s.repetitionDraw(b.key, int(b.halfmoveClock), ply) {
			return 0
		}
		best := -infinity
		for _, m := range moves {
			child := b.makeMove(m)
			if score := -referenceQuiesceAt(s, &child, ply+1); score > best {
				best = score
			}
		}
		return best
	}

	if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() ||
		s.repetitionDraw(b.key, int(b.halfmoveClock), ply) {
		return 0
	}
	best := b.evaluateForSideToMove()
	for _, m := range b.legalMoves(buf[:0]) {
		if _, promotion := m.Promotion(); !m.IsCapture() && !promotion {
			continue
		}
		child := b.makeMove(m)
		if score := -referenceQuiesceAt(s, &child, ply+1); score > best {
			best = score
		}
	}
	return best
}
