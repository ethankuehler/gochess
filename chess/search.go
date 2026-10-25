package chess

import (
	"context"
	"time"
)

// MateScore is the score of a checkmate at the root. Scores within 128 points of
// ±MateScore encode forced mates and describe the distance to mate.
const MateScore = 1_000_000

const (
	mateScore = MateScore
	maxPly    = 128
	infinity  = mateScore + 1

	// searchCheckInterval controls how often the searcher polls the context
	// and the wall-clock deadline. Node and depth limits are checked on every
	// node; cancellation is checked often enough to keep stop responsive.
	searchCheckInterval = 256

	// maxRepeatHistory bounds the game history copied into a search. A
	// position with a halfmove clock of 100 or more is already a draw, so at
	// most 99 earlier positions can still take part in a repetition.
	maxRepeatHistory = 100
)

// SearchLimits describes the bounds of one search. Zero values mean "no limit"
// except for MaxDepth, where zero also means "no explicit depth limit" and the
// searcher deepens until the deadline, the node limit, or cancellation.
type SearchLimits struct {
	MaxDepth       int
	MaxNodes       uint64
	Deadline       time.Time           // hard wall-clock deadline; zero means none
	SoftDeadline   time.Time           // stop between iterations after this time; zero means none
	RootMoves      []Move              // nil means every legal move; non-nil restricts the root
	GameHistory    []PositionKey       // keys of positions before the root, oldest first
	TT             *TranspositionTable // nil disables the transposition table
	CollectMetrics bool                // honored only in a MetricsEnabled build
}

// SearchMetrics contains runtime counters and inclusive wall-clock durations
// for one search. Move generation, legality filtering, evaluation, and
// search-edge copy-make are timed separately, so the phases do not overlap.
// They do not cover all search work: draw checks, move ordering, PV
// maintenance, and the info callback are unmeasured.
//
// MakeMoveCalls counts every copy-make performed during the search, including
// the copies legal move generation makes to test king safety.
// SearchMakeMoveCalls is the subset made directly by the search tree. The time
// spent on the legality-filter copies is included in LegalFilterNS because
// separating it from the attack test would require timing every individual
// copy.
type SearchMetrics struct {
	ElapsedNS           int64  `json:"elapsedNs"`
	MoveGenCalls        uint64 `json:"moveGenCalls"`
	PseudoMoves         uint64 `json:"pseudoMoves"`
	LegalMoves          uint64 `json:"legalMoves"`
	EvaluationCalls     uint64 `json:"evaluationCalls"`
	MakeMoveCalls       uint64 `json:"makeMoveCalls"`
	SearchMakeMoveCalls uint64 `json:"searchMakeMoveCalls"`
	BetaCutoffs         uint64 `json:"betaCutoffs"`
	MoveGenNS           int64  `json:"moveGenNs"`
	LegalFilterNS       int64  `json:"legalFilterNs"`
	EvaluationNS        int64  `json:"evaluationNs"`
	SearchMakeMoveNS    int64  `json:"searchMakeMoveNs"`
}

// SearchInfo is the result of one completed iterative-deepening iteration.
type SearchInfo struct {
	Depth    int
	SelDepth int
	Score    int // centipawns from the side to move's point of view
	Nodes    uint64
	Time     time.Duration
	PV       []Move
}

// SearchResult is the outcome of one search.
type SearchResult struct {
	BestMove   Move           // valid only when HasMove is true
	PonderMove Move           // second PV move, when one exists
	Score      int            // centipawns from the side to move's point of view
	HasMove    bool           // false for checkmate and stalemate
	Draw       bool           // root is already a fifty-move, threefold, or insufficient-material draw
	Depth      int            // deepest completed iteration, in plies
	SelDepth   int            // deepest ply reached
	Nodes      uint64         // negamax nodes visited, excluding the root
	PV         []Move         // principal variation from the last completed iteration
	TT         TTStats        // transposition-table activity for this search
	Metrics    *SearchMetrics // nil unless CollectMetrics is true and MetricsEnabled
}

// Search returns the best move for the side to move at the given depth in plies.
// It panics if depth is less than 1. The attack tables must have been built with
// BuildAllAttacks or the individual builders.
//
// Search allocates a fresh transposition table for each call, so repeated calls
// are deterministic. Shallower searches skip the allocation: at one or two
// plies there are too few transpositions to repay it. Use SearchWithLimits with
// SearchLimits.TT to reuse a table across searches.
func (b *Position) Search(depth int) SearchResult {
	if depth < 1 {
		panic("chess: search depth must be at least 1")
	}
	limits := SearchLimits{MaxDepth: depth}
	if depth >= 3 {
		limits.TT = NewTranspositionTable(DefaultHashMB)
	}
	return b.SearchWithLimits(context.Background(), limits, nil)
}

// SearchWithLimits performs iterative-deepening alpha-beta search subject to
// limits. It reports every completed iteration through info, which may be nil.
// Cancellation through ctx, the deadline, and the node limit are all checked
// during the search; a stopped search returns the best move and PV from the
// last completed iteration, or a legal fallback move if no iteration completed.
//
// The attack tables must have been built with BuildAllAttacks,
// BuildAllAttacksFromFS, or the individual builders.
func (b *Position) SearchWithLimits(ctx context.Context, limits SearchLimits, info func(SearchInfo)) (result SearchResult) {
	if ctx == nil {
		ctx = context.Background()
	}
	ensureAttackTables()

	var metrics *SearchMetrics
	var metricsStart time.Time
	if limits.CollectMetrics && MetricsEnabled {
		metrics = &SearchMetrics{}
		metricsStart = time.Now()
		defer func() {
			metrics.ElapsedNS = time.Since(metricsStart).Nanoseconds()
			result.Metrics = metrics
		}()
	}
	s := searcher{
		ctx:      ctx,
		deadline: limits.Deadline,
		maxNodes: limits.MaxNodes,
		tt:       limits.TT,
		metrics:  metrics,
	}
	defer func() {
		result.TT = s.ttStats
	}()
	s.tt.newSearch()

	var buf [256]Move
	moves := s.legalMoves(b, buf[:0])
	if len(moves) == 0 {
		if b.InCheck() {
			result.Score = -mateScore
		}
		return result
	}

	if limits.RootMoves != nil {
		filtered := make([]Move, 0, len(limits.RootMoves))
		seen := make(map[Move]bool, len(limits.RootMoves))
		for _, candidate := range limits.RootMoves {
			for _, legal := range moves {
				if legal == candidate && !seen[legal] {
					filtered = append(filtered, legal)
					seen[legal] = true
					break
				}
			}
		}
		moves = filtered
		if len(moves) == 0 {
			return result
		}
	}

	maxDepth := limits.MaxDepth
	if maxDepth < 1 || maxDepth > maxPly {
		maxDepth = maxPly
	}

	start := time.Now()

	// Only positions since the last irreversible move can repeat, and a
	// halfmove clock of 100 or more is already a draw, so the useful history
	// is bounded.
	history := limits.GameHistory
	if len(history) > int(b.halfmoveClock) {
		history = history[len(history)-int(b.halfmoveClock):]
	}
	if len(history) > maxRepeatHistory {
		history = history[len(history)-maxRepeatHistory:]
	}
	for i, key := range history {
		s.keys[i] = uint64(key)
	}
	s.pathBase = len(history)

	if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() || s.repetitionDraw(b.key, int(b.halfmoveClock), 0) {
		// The root is already a claimable draw, but the UCI protocol still
		// requires a legal move when the GUI asks for one.
		result.BestMove = moves[0]
		result.Score = 0
		result.HasMove = true
		result.Draw = true
		result.PV = []Move{moves[0]}
		return result
	}
	s.keys[s.pathBase] = b.key

	// A persistent table can supply a root move from an earlier search. The
	// move is only used for ordering; searchmoves filtering still wins.
	var rootHash Move
	if s.tt != nil {
		s.ttStats.Probes++
		if entry, ok := s.tt.probe(b.key); ok {
			s.ttStats.Hits++
			rootHash = entry.move
		}
	}

	fallback := moves[0]
	ordered := append([]Move(nil), moves...)
	promoteMove(ordered, rootHash)
	completedDepth := 0
	var bestMove Move
	var bestScore int
	var bestPV []Move

	for depth := 1; depth <= maxDepth; depth++ {
		if s.shouldStop() {
			break
		}

		s.pvLength[0] = 0
		alpha, beta := -infinity, infinity
		iterBest := Move{}
		iterScore := -infinity
		var iterPV []Move
		found := false
		aborted := false

		for _, m := range ordered {
			if s.shouldStop() {
				aborted = true
				break
			}
			child := s.makeMove(b, m)
			score := -s.negamax(child, depth-1, -beta, -alpha, 1)
			if s.stopped {
				aborted = true
				break
			}
			if score > iterScore {
				iterScore = score
				iterBest = m
				found = true
				iterPV = make([]Move, 1, 1+s.pvLength[1])
				iterPV[0] = m
				iterPV = append(iterPV, s.pvTable[1][:s.pvLength[1]]...)
			}
			if score > alpha {
				alpha = score
			}
		}

		if aborted || !found {
			break
		}

		completedDepth = depth
		bestMove, bestScore, bestPV = iterBest, iterScore, iterPV
		result.BestMove = iterBest
		result.Score = iterScore
		result.HasMove = true
		result.Depth = depth
		result.SelDepth = s.seldepth
		result.Nodes = s.nodes
		result.PV = append([]Move(nil), iterPV...)
		if len(iterPV) >= 2 {
			result.PonderMove = iterPV[1]
		}
		if s.tt != nil && b.halfmoveClock < ttMaxHalfmoveClock {
			// Keep the completed root result for the next search. The score is
			// exact at this depth, and the move is the best ordering hint a
			// later search on the same position can get. The clock guard
			// matches negamax: a score found near the fifty-move horizon must
			// not leak into a fresh position with the same key.
			s.tt.store(b.key, depth, scoreToTT(iterScore, 0), ttExact, iterBest)
			s.ttStats.Stores++
		}

		if info != nil {
			info(SearchInfo{
				Depth:    depth,
				SelDepth: s.seldepth,
				Score:    iterScore,
				Nodes:    s.nodes,
				Time:     time.Since(start),
				PV:       append([]Move(nil), iterPV...),
			})
		}

		promoteMove(ordered, iterBest)
		if !limits.SoftDeadline.IsZero() && !time.Now().Before(limits.SoftDeadline) {
			break
		}
	}

	if completedDepth == 0 {
		// The search was stopped before a full iteration. A legal move is
		// still required for every go command.
		result.BestMove = fallback
		result.Score = 0
		result.HasMove = true
		result.Depth = 0
		result.SelDepth = s.seldepth
		result.Nodes = s.nodes
		result.PV = []Move{fallback}
		return result
	}

	result.BestMove, result.Score, result.PV = bestMove, bestScore, bestPV
	result.Nodes = s.nodes
	result.SelDepth = s.seldepth
	return result
}

// searcher carries the node counter, limits, and PV table for one search.
type searcher struct {
	ctx      context.Context
	deadline time.Time
	maxNodes uint64
	nodes    uint64
	stopped  bool
	seldepth int
	metrics  *SearchMetrics

	tt      *TranspositionTable
	ttStats TTStats

	// pathBase is the number of game-history keys prepended to keys. The key
	// for the node at ply p lives at keys[pathBase+p], which makes repetition
	// checks a short backwards scan.
	pathBase int
	keys     [maxRepeatHistory + maxPly + 1]uint64

	pvTable  [maxPly + 1][maxPly + 1]Move
	pvLength [maxPly + 1]int
}

// repetitionDraw reports whether key has already occurred twice in the last
// halfmoveClock positions of the current line, which would make this the third
// occurrence and therefore a draw. The current node's key has not been written
// yet, so only earlier positions are compared.
func (s *searcher) repetitionDraw(key uint64, halfmoveClock, ply int) bool {
	if halfmoveClock <= 0 {
		return false
	}
	index := s.pathBase + ply
	available := index
	if halfmoveClock < available {
		available = halfmoveClock
	}
	occurrences := 0
	for offset := 1; offset <= available; offset++ {
		if s.keys[index-offset] != key {
			continue
		}
		occurrences++
		if occurrences >= 2 {
			return true
		}
	}
	return false
}

// tick counts a node and checks the node limit and, periodically, the context
// and the wall-clock deadline.
func (s *searcher) tick() bool {
	s.nodes++
	if s.maxNodes > 0 && s.nodes >= s.maxNodes {
		s.stopped = true
		return true
	}
	if s.nodes%searchCheckInterval == 0 {
		return s.shouldStop()
	}
	return false
}

// shouldStop reports whether the search should stop. It is cheap enough to call
// at root move boundaries and every searchCheckInterval nodes.
func (s *searcher) shouldStop() bool {
	if s.stopped {
		return true
	}
	if s.maxNodes > 0 && s.nodes >= s.maxNodes {
		s.stopped = true
		return true
	}
	if s.ctx != nil {
		select {
		case <-s.ctx.Done():
			s.stopped = true
			return true
		default:
		}
	}
	if !s.deadline.IsZero() && !time.Now().Before(s.deadline) {
		s.stopped = true
		return true
	}
	return false
}

// negamax returns the score of the position for the side to move. ply is the
// distance from the root and is used to prefer shorter mates and to index the
// PV table. Position is passed by value: it is small, and the value parameter
// keeps the copy-make child on the stack instead of forcing a heap allocation
// per node.
func (s *searcher) negamax(b Position, depth, alpha, beta, ply int) int {
	if s.tick() {
		return 0
	}
	if ply > s.seldepth {
		s.seldepth = ply
	}
	if ply < len(s.pvLength) {
		s.pvLength[ply] = 0
	}

	if depth == 0 {
		// Checkmate at the horizon still has to be detected, otherwise a
		// depth-1 search would miss mate in one. It also takes precedence over
		// a draw claim, because checkmate ends the game immediately. The move
		// buffer is scoped here so the common quiet-leaf case does not zero it.
		if b.InCheck() {
			var buf [256]Move
			if len(s.legalMoves(&b, buf[:0])) == 0 {
				return -mateScore + ply
			}
		}
		if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() || s.repetitionDraw(b.key, int(b.halfmoveClock), ply) {
			return 0
		}
		return s.evaluate(&b)
	}

	alphaOrig := alpha
	inCheck := b.InCheck()
	ttUsable := s.tt != nil && b.halfmoveClock < ttMaxHalfmoveClock
	var hashMove Move

	// When the side to move is not in check, the draw checks can run before
	// move generation, which lets a table cutoff skip that work entirely. When
	// it is in check, mate has to be ruled out first: checkmate beats a fifty-
	// move or repetition claim on the same node.
	if !inCheck {
		if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() || s.repetitionDraw(b.key, int(b.halfmoveClock), ply) {
			return 0
		}
		if ttUsable {
			if score, cutoff, move := s.probeTT(b.key, depth, alpha, beta, ply); cutoff {
				return score
			} else {
				hashMove = move
			}
		}
	}

	var buf, ordered [256]Move
	moves := s.legalMoves(&b, buf[:0])
	if len(moves) == 0 {
		if inCheck {
			return -mateScore + ply
		}
		return 0 // stalemate
	}

	if inCheck {
		// The fifty-move rule and a third repetition are claimable draws, but
		// a checkmate on the same node still wins.
		if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() || s.repetitionDraw(b.key, int(b.halfmoveClock), ply) {
			return 0
		}
		if ttUsable {
			if score, cutoff, move := s.probeTT(b.key, depth, alpha, beta, ply); cutoff {
				return score
			} else {
				hashMove = move
			}
		}
	}
	s.keys[s.pathBase+ply] = b.key

	best := -infinity
	var bestMove Move
	for _, m := range orderMoves(moves, ordered[:0], hashMove) {
		child := s.makeMove(&b, m)
		score := -s.negamax(child, depth-1, -beta, -alpha, ply+1)
		if s.stopped {
			return 0
		}
		if score > best {
			best = score
			bestMove = m
			s.updatePV(ply, m)
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			if MetricsEnabled && s.metrics != nil {
				s.metrics.BetaCutoffs++
			}
			break
		}
	}

	if ttUsable {
		s.storeTT(b.key, depth, best, alphaOrig, beta, ply, bestMove)
	}
	return best
}

// probeTT looks up key and applies the standard alpha-beta table cutoff. A
// cutoff is only taken at null-window nodes; principal-variation nodes keep
// searching so the PV is built from real lines rather than a cached score. The
// returned move is useful for ordering even when the entry is too shallow.
func (s *searcher) probeTT(key uint64, depth, alpha, beta, ply int) (score int, cutoff bool, move Move) {
	s.ttStats.Probes++
	entry, ok := s.tt.probe(key)
	if !ok {
		return 0, false, Move{}
	}
	s.ttStats.Hits++
	move = entry.move
	if int(entry.depth) < depth || beta-alpha <= 1 {
		return 0, false, move
	}

	score = scoreFromTT(entry.score, ply)
	switch entry.bound {
	case ttExact:
		s.ttStats.Cutoffs++
		return score, true, move
	case ttLower:
		if score >= beta {
			s.ttStats.Cutoffs++
			return score, true, move
		}
	case ttUpper:
		if score <= alpha {
			s.ttStats.Cutoffs++
			return score, true, move
		}
	}
	return 0, false, move
}

// storeTT records the result of a completed node. Fail-soft alpha-beta can
// return a score outside the search window, so the original alpha and beta
// decide whether the result is exact or a bound.
func (s *searcher) storeTT(key uint64, depth, best, alphaOrig, beta, ply int, move Move) {
	if s.tt == nil || s.stopped {
		return
	}
	bound := ttExact
	switch {
	case best <= alphaOrig:
		bound = ttUpper
	case best >= beta:
		bound = ttLower
	}
	s.tt.store(key, depth, scoreToTT(best, ply), bound, move)
	s.ttStats.Stores++
}

// updatePV stores the line beginning with m at ply. The child line is copied
// from ply+1; pvLength is bounded so the table index stays in range even at the
// maximum depth.
func (s *searcher) updatePV(ply int, m Move) {
	s.pvTable[ply][0] = m
	if ply >= maxPly {
		s.pvLength[ply] = 1
		return
	}
	n := s.pvLength[ply+1]
	if n > maxPly-ply-1 {
		n = maxPly - ply - 1
	}
	copy(s.pvTable[ply][1:], s.pvTable[ply+1][:n])
	s.pvLength[ply] = 1 + n
}

// promoteMove moves best to the front of moves, preserving the relative order
// of the others. It is used to search the previous iteration's best move first.
func promoteMove(moves []Move, best Move) {
	for i, m := range moves {
		if m != best {
			continue
		}
		copy(moves[1:i+1], moves[:i])
		moves[0] = best
		return
	}
}

// orderMoves puts the transposition-table move first, then captures, en
// passant, and promotions, then quiet moves. It writes into scratch and returns
// the used slice; ordering changes the number of nodes visited but never the
// search score. A hash move that is not in the legal list is ignored.
func orderMoves(moves, scratch []Move, hashMove Move) []Move {
	out := scratch[:0]
	if hashMove != (Move{}) {
		for _, m := range moves {
			if m == hashMove {
				out = append(out, m)
				break
			}
		}
	}
	for _, m := range moves {
		if m == hashMove {
			continue
		}
		if _, promotion := m.Promotion(); m.IsCapture() || promotion {
			out = append(out, m)
		}
	}
	for _, m := range moves {
		if m == hashMove {
			continue
		}
		if _, promotion := m.Promotion(); !m.IsCapture() && !promotion {
			out = append(out, m)
		}
	}
	return out
}

// ScoreToMate converts a mate score into a distance in moves. ok is false for
// ordinary evaluations. A positive distance means the side to move delivers
// mate; a negative distance means it is being mated.
func ScoreToMate(score int) (moves int, ok bool) {
	switch {
	case score > mateScore-maxPly:
		return (mateScore - score + 1) / 2, true
	case score < -mateScore+maxPly:
		return -((mateScore + score + 1) / 2), true
	default:
		return 0, false
	}
}
