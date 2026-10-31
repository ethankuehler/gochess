package chess

import (
	"context"
	"sync/atomic"
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

	// Root aspiration windows. The first window is +/- aspirationDelta around
	// the previous score and doubles on a fail, falling back to the full window
	// after aspirationMaxRetries attempts.
	aspirationDelta      = 25
	aspirationMaxRetries = 4

	// futilityMargin is the per-depth centipawn margin for futility pruning.
	futilityMargin = 100

	// maxRepeatHistory bounds the game history copied into a search. A
	// position with a halfmove clock of 100 or more is already a draw, so at
	// most 99 earlier positions can still take part in a repetition.
	maxRepeatHistory = 100
)

// SearchClock holds the wall-clock deadlines for one search. The zero value
// means no deadline. It is safe to update while a search is running, which is
// how pondering starts without a clock and installs the move budget when the
// GUI sends ponderhit.
//
// Deadlines are stored as Unix nanoseconds, so the monotonic clock reading of a
// time.Time is dropped. That is fine for a move deadline: the search only needs
// to compare wall-clock instants, and a single atomic load keeps the check
// cheap on the node path.
type SearchClock struct {
	soft atomic.Int64 // Unix nanoseconds; 0 means none
	hard atomic.Int64
}

// SetDeadlines installs the soft and hard deadlines. A zero time clears that
// deadline. The search reads them after every completed iteration and every
// searchCheckInterval nodes, so an update takes effect at the next check.
func (c *SearchClock) SetDeadlines(soft, hard time.Time) {
	c.soft.Store(deadlineNanos(soft))
	c.hard.Store(deadlineNanos(hard))
}

// Deadlines returns the deadlines currently installed. A zero time means that
// deadline is unset.
func (c *SearchClock) Deadlines() (soft, hard time.Time) {
	return timeFromNanos(c.soft.Load()), timeFromNanos(c.hard.Load())
}

func deadlineNanos(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

func timeFromNanos(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// softPassed reports whether the soft deadline has been reached.
func (c *SearchClock) softPassed() bool {
	d := c.soft.Load()
	return d != 0 && time.Now().UnixNano() >= d
}

// hardPassed reports whether the hard deadline has been reached.
func (c *SearchClock) hardPassed() bool {
	d := c.hard.Load()
	return d != 0 && time.Now().UnixNano() >= d
}

// SearchLimits describes the bounds of one search. Zero values mean "no limit"
// except for MaxDepth, where zero also means "no explicit depth limit" and the
// searcher deepens until the deadline, the node limit, or cancellation.
type SearchLimits struct {
	MaxDepth       int
	MaxNodes       uint64              // counts negamax and quiescence nodes; zero means none
	Deadline       time.Time           // hard wall-clock deadline; zero means none
	SoftDeadline   time.Time           // stop between iterations after this time; zero means none
	Clock          *SearchClock        // overrides the two deadlines; may be updated while searching
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
// The counters and timings include quiescence work at depth-zero nodes: legal
// generation, evaluation, copy-make, and beta cutoffs are all included in the
// same fields. PseudoMoves counts directly emitted legal moves plus king and
// en-passant candidates before rejection. MakeMoveCalls counts every copy-make
// performed during the search, including king and en-passant validation in
// legal move generation. SearchMakeMoveCalls is the subset made directly by the
// search tree, including quiescence nodes. The time spent on special-move
// validation is included in LegalFilterNS.
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
	SelDepth   int            // deepest ply reached, including quiescence
	Nodes      uint64         // negamax and quiescence nodes visited, excluding the root
	PV         []Move         // principal variation from the last completed iteration
	TT         TTStats        // transposition-table activity for this search
	Metrics    *SearchMetrics // nil unless CollectMetrics is true and MetricsEnabled
}

// Search returns the best move for the side to move at the given depth in plies.
// It panics if depth is less than 1. The attack tables must have been built with
// BuildAllAttacks or the individual builders.
//
// Leaves are resolved by a quiescence search over captures, en passant, and
// promotions, so captures just past the nominal horizon still affect the score.
// Depth remains the deepest completed iterative-deepening iteration and does not
// include quiescence plies.
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
// Depth-zero leaves are resolved by quiesce, so the node limit, deadlines, and
// cancellation all include quiescence work. Cancellation through ctx, the
// deadline, and the node limit are checked during the search; a stopped search
// returns the best move and PV from the last completed iteration, or a legal
// fallback move if no iteration completed.
//
// The attack tables must have been built with BuildAllAttacks,
// BuildAllAttacksFromFS, or the individual builders.
func (b *Position) SearchWithLimits(ctx context.Context, limits SearchLimits, info func(SearchInfo)) SearchResult {
	return b.searchWithOptions(ctx, limits, info, defaultSearchOptions())
}

// searchOptions selects the selective-search features for one search. Production
// search enables all of them; tests use the flags to prove a guard fires and to
// compare a feature against the full-width behavior.
type searchOptions struct {
	pvs        bool
	aspiration bool
	nullMove   bool
	lmr        bool
	futility   bool
}

// defaultSearchOptions enables every feature used in play.
func defaultSearchOptions() searchOptions {
	return searchOptions{pvs: true, aspiration: true, nullMove: true, lmr: true, futility: true}
}

// searchWithOptions performs the search described by SearchWithLimits with the
// given feature switches.
func (b *Position) searchWithOptions(ctx context.Context, limits SearchLimits, info func(SearchInfo), opts searchOptions) (result SearchResult) {
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
	clock := limits.Clock
	if clock == nil {
		clock = new(SearchClock)
		clock.SetDeadlines(limits.SoftDeadline, limits.Deadline)
	}
	s := searcher{
		ctx:      ctx,
		clock:    clock,
		maxNodes: limits.MaxNodes,
		tt:       limits.TT,
		metrics:  metrics,
		opts:     opts,
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
	ordered = s.orderMoves(b, ordered, rootHash, 0)
	completedDepth := 0
	var bestMove Move
	var bestScore int
	var bestPV []Move

	for depth := 1; depth <= maxDepth; depth++ {
		if s.shouldStop() {
			break
		}

		// Aspiration windows start narrow around the previous iteration's
		// score and widen on a fail. Deeper iterations near a mate and the
		// first few depths use the full window.
		alpha, beta := -infinity, infinity
		if s.opts.aspiration && depth >= 4 && completedDepth > 0 && !nearMate(bestScore) {
			alpha = max(bestScore-aspirationDelta, -infinity)
			beta = min(bestScore+aspirationDelta, infinity)
		}

		var iterBest Move
		var iterScore int
		var iterPV []Move
		aborted := false
		completed := false
		for attempt := 0; ; attempt++ {
			iterBest, iterScore, iterPV, aborted = s.searchRoot(b, ordered, depth, alpha, beta)
			if aborted {
				break
			}
			if iterScore > alpha && iterScore < beta {
				completed = true
				break
			}
			// A fail means the window missed the score. Widen it and search
			// this depth again; the last attempt always uses the full window.
			promoteMove(ordered, iterBest)
			if attempt >= aspirationMaxRetries {
				alpha, beta = -infinity, infinity
			} else if iterScore <= alpha {
				alpha = max(alpha-(beta-alpha), -infinity)
			} else {
				beta = min(beta+(beta-alpha), infinity)
			}
		}
		if aborted || !completed {
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
		if s.clock.softPassed() {
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

// searchRoot searches every root move at the given depth and window. It returns
// the best move, score, and PV, and reports whether the search was stopped. A
// root move that is scored as a repetition is not searched, so it contributes a
// one-move PV and never inherits the previous root move's line. Only a move
// searched with the full window contributes a PV tail.
func (s *searcher) searchRoot(b *Position, ordered []Move, depth, alpha, beta int) (best Move, score int, pv []Move, aborted bool) {
	score = -infinity
	wideWindow := true // the first searched move gets the full window
	for _, m := range ordered {
		if s.shouldStop() {
			return Move{}, 0, nil, true
		}
		s.pvLength[1] = 0
		child := s.makeMove(b, m)
		moveScore := 0
		searched := false
		wide := false
		if !s.repeatsEarlier(child.key, int(child.halfmoveClock), 0) {
			searched = true
			if !s.opts.pvs || wideWindow {
				moveScore = -s.negamax(child, depth-1, -beta, -alpha, 1, true)
				wideWindow = false
				wide = true
			} else {
				moveScore = -s.negamax(child, depth-1, -alpha-1, -alpha, 1, true)
				if moveScore > alpha && moveScore < beta {
					moveScore = -s.negamax(child, depth-1, -beta, -alpha, 1, true)
					wide = true
				}
			}
		}
		if s.stopped {
			return Move{}, 0, nil, true
		}
		if moveScore > score {
			score = moveScore
			best = m
			pv = append(pv[:0], m)
			if searched && wide {
				pv = append(pv, s.pvTable[1][:s.pvLength[1]]...)
			}
		}
		if moveScore > alpha {
			alpha = moveScore
		}
	}
	return best, score, pv, false
}

// nearMate reports whether score is close enough to a mate score that an
// aspiration window would be meaningless.
func nearMate(score int) bool {
	return score >= mateScoreTTThreshold || score <= -mateScoreTTThreshold
}

// mateWindow reports whether either bound is a mate score, in which case
// selective pruning must not run.
func mateWindow(alpha, beta int) bool {
	return beta >= mateScoreTTThreshold || alpha <= -mateScoreTTThreshold
}

// futilityAllowed reports whether the node may compute a static score for
// futility pruning and razoring. Only frontier scout nodes qualify.
func (s *searcher) futilityAllowed(depth int, pv, inCheck bool, alpha, beta int) bool {
	return s.opts.futility && !pv && !inCheck && depth <= 2 && !mateWindow(alpha, beta)
}

// futilityPrunes reports whether a quiet move can be skipped: the static score
// plus the margin still cannot reach alpha. The first move, the hash move,
// captures, and promotions are never pruned.
func (s *searcher) futilityPrunes(m, hashMove Move, depth, staticEval, alpha int, haveStaticEval bool, best int) bool {
	if !haveStaticEval || best <= -infinity || m == hashMove {
		return false
	}
	if _, promotion := m.Promotion(); m.IsCapture() || promotion {
		return false
	}
	return staticEval+futilityMargin*depth <= alpha
}

// lateMoveReduction returns the reduction to apply to a quiet scout move, or 0
// when the guards reject it. Only late quiet moves that do not give check are
// reduced, and never at a PV node, so the principal variation is searched at
// full depth. A reduced move that beats alpha is re-searched at full depth by
// the caller, so the reduction can only cost nodes, never accuracy.
func (s *searcher) lateMoveReduction(child *Position, m Move, depth, moveIndex int, pv bool, alpha, beta int) int {
	if !s.opts.lmr || pv || depth < 3 || moveIndex < 3 || mateWindow(alpha, beta) {
		return 0
	}
	if m.IsCapture() {
		return 0
	}
	if _, promotion := m.Promotion(); promotion {
		return 0
	}
	if child.InCheck() {
		return 0
	}
	reduction := 1 + (depth-3)/4 + moveIndex/8
	if limit := depth - 2; reduction > limit {
		reduction = limit
	}
	if reduction > 3 {
		reduction = 3
	}
	return reduction
}

// nullMoveAllowed reports whether null-move pruning may run at this node. It is
// deliberately conservative: scout nodes only, not in check, not immediately
// after another null move, deep enough, outside a mate window, and only when the
// side to move has a non-pawn piece. Keeping the guard in one place makes each
// condition testable.
func (s *searcher) nullMoveAllowed(b *Position, depth int, pv, inCheck, nullAllowed bool, alpha, beta int) bool {
	return s.opts.nullMove && !pv && !inCheck && nullAllowed && depth >= 5 &&
		!mateWindow(alpha, beta) && b.hasNonPawnMaterial(b.sideToMove())
}

// searcher carries the node counter, limits, and PV table for one search.
type searcher struct {
	opts searchOptions

	ctx      context.Context
	clock    *SearchClock
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

	// killers holds the two most recent quiet moves that caused a beta cutoff
	// at each ply. history scores quiet moves by how often they cut off, split
	// by the side that played them.
	killers [maxPly + 1][2]Move
	history [2][64][64]int
}

// repeatsEarlier reports whether key has already occurred once in the last
// halfmoveClock positions of the game history and search path. A move into such
// a position is scored as a draw: the opponent can usually repeat it a third
// time, and treating the repetition as a draw keeps a winning side from
// shuffling instead of converting. The current node's key is at
// pathBase+ply; the child that would be reached by the candidate move belongs at
// pathBase+ply+1 and has not been written yet.
func (s *searcher) repeatsEarlier(key uint64, halfmoveClock, ply int) bool {
	if halfmoveClock <= 0 {
		return false
	}
	index := s.pathBase + ply + 1
	available := index
	if halfmoveClock < available {
		available = halfmoveClock
	}
	for offset := 1; offset <= available; offset++ {
		if s.keys[index-offset] == key {
			return true
		}
	}
	return false
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
	if s.clock != nil && s.clock.hardPassed() {
		s.stopped = true
		return true
	}
	return false
}

// negamax returns the score of the position for the side to move. ply is the
// distance from the root and is used to prefer shorter mates and to index the
// PV table. Depth-zero nodes are resolved by quiesce, which counts its own node,
// so tick is reached only for depth-positive nodes. Position is passed by value:
// it is small, and the value parameter keeps the copy-make child on the stack
// instead of forcing a heap allocation per node.
func (s *searcher) negamax(b Position, depth, alpha, beta, ply int, nullAllowed bool) int {
	if depth <= 0 {
		return s.quiesce(b, alpha, beta, ply)
	}
	if s.tick() {
		return 0
	}
	if ply > s.seldepth {
		s.seldepth = ply
	}
	if ply < len(s.pvLength) {
		s.pvLength[ply] = 0
	}
	// Record this node's key before anything recurses. Repetition checks below
	// scan earlier entries only, so the write can happen up front, and a null
	// move needs the parent key already in place.
	s.keys[s.pathBase+ply] = b.key

	// pv distinguishes principal-variation nodes from null-window (scout)
	// nodes. With PVS the first move of a PV node is searched with the full
	// window and later moves are scouted, so window width still identifies the
	// node type. Table cutoffs and all unsound pruning are limited to scouts.
	pv := s.opts.pvs && beta-alpha > 1

	alphaOrig := alpha
	inCheck := b.InCheck()
	ttUsable := s.tt != nil && b.halfmoveClock < ttMaxHalfmoveClock
	var hashMove Move
	var staticEval int
	haveStaticEval := false

	// When the side to move is not in check, the draw checks can run before
	// move generation, which lets a table cutoff skip that work entirely. When
	// it is in check, mate has to be ruled out first: checkmate beats a fifty-
	// move or repetition claim on the same node.
	if !inCheck {
		if b.IsFiftyMoveDraw() || b.IsInsufficientMaterial() || s.repetitionDraw(b.key, int(b.halfmoveClock), ply) {
			return 0
		}
		if ttUsable {
			if score, cutoff, move := s.probeTT(b.key, depth, alpha, beta, ply, pv); cutoff {
				return score
			} else {
				hashMove = move
			}
		}
		if s.nullMoveAllowed(&b, depth, pv, false, nullAllowed, alpha, beta) {
			// Shallow null searches cost more than they save in this engine:
			// at depth 6 they roughly double the kiwipete subtree, while from
			// depth 7 they are a clear win. Require a deep-enough node.
			reduction := 3
			child := s.makeNullMove(&b)
			score := -s.negamax(child, depth-1-reduction, -beta, -beta+1, ply+1, false)
			if s.stopped {
				return 0
			}
			if score >= beta {
				// The cutoff is a lower bound proved by giving the opponent a
				// free move, so an unproven mate score must not escape.
				if score >= mateScoreTTThreshold {
					score = beta
				}
				if ttUsable {
					s.storeTT(b.key, depth, score, alpha, beta, ply, Move{})
				}
				return score
			}
		}
		// Frontier nodes get a static score once for futility pruning. It is
		// limited to scout nodes, so the PV is unaffected. Razoring was tried
		// here as well and measured as a node regression in this engine; only
		// futility is kept.
		if s.futilityAllowed(depth, pv, false, alpha, beta) {
			staticEval = s.evaluate(&b)
			haveStaticEval = true
		}
	}

	var buf [256]Move
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
			if score, cutoff, move := s.probeTT(b.key, depth, alpha, beta, ply, pv); cutoff {
				return score
			} else {
				hashMove = move
			}
		}
	}
	best := -infinity
	var bestMove Move
	us := b.sideToMove()
	first := true
	moveIndex := 0
	for _, m := range s.orderMoves(&b, moves, hashMove, ply) {
		// A quiet move that cannot lift the static score to alpha is not worth
		// searching at a frontier scout node. Captures and promotions are
		// still searched, and the first move is never pruned.
		if s.futilityPrunes(m, hashMove, depth, staticEval, alpha, haveStaticEval, best) {
			first = false
			moveIndex++
			continue
		}
		// Reset the child's PV first: a move skipped by repeatsEarlier is not
		// searched, and updatePV must not copy the previous sibling's line.
		if ply+1 < len(s.pvLength) {
			s.pvLength[ply+1] = 0
		}
		child := s.makeMove(&b, m)
		score := 0
		if s.repeatsEarlier(child.key, int(child.halfmoveClock), ply) {
			score = 0
		} else if !s.opts.pvs || (first && pv) {
			// The first move of a PV node gets the full window. At a null-window
			// node every move is already searched with a null window, so only
			// PV nodes reach the scout branch below.
			score = -s.negamax(child, depth-1, -beta, -alpha, ply+1, true)
		} else if reduction := s.lateMoveReduction(&child, m, depth, moveIndex, pv, alpha, beta); reduction > 0 {
			// Search a late quiet move shallower first; if it still beats
			// alpha, repeat the search at full depth before trusting it.
			score = -s.negamax(child, depth-1-reduction, -alpha-1, -alpha, ply+1, true)
			if score > alpha {
				score = -s.negamax(child, depth-1, -beta, -alpha, ply+1, true)
			}
		} else {
			// Scout with a null window and re-search the full window when the
			// move improves alpha. At a null-window node score > alpha implies
			// score >= beta, so this branch only does work at PV nodes.
			score = -s.negamax(child, depth-1, -alpha-1, -alpha, ply+1, true)
			if pv && score > alpha && score < beta {
				score = -s.negamax(child, depth-1, -beta, -alpha, ply+1, true)
			}
		}
		first = false
		moveIndex++
		if s.stopped {
			return 0
		}
		if score > best {
			best = score
			bestMove = m
			if pv {
				s.updatePV(ply, m)
			}
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			if MetricsEnabled && s.metrics != nil {
				s.metrics.BetaCutoffs++
			}
			if _, promotion := m.Promotion(); !m.IsCapture() && !promotion {
				s.recordQuietCutoff(us, m, depth, ply)
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
func (s *searcher) probeTT(key uint64, depth, alpha, beta, ply int, pv bool) (score int, cutoff bool, move Move) {
	s.ttStats.Probes++
	entry, ok := s.tt.probe(key)
	if !ok {
		return 0, false, Move{}
	}
	s.ttStats.Hits++
	move = entry.move
	if int(entry.depth) < depth || pv {
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

// Move-ordering scores. The gaps leave room for the components to be added
// without one class ever outranking another by accident.
const (
	orderHashMove   = 1 << 30
	orderCapture    = 1 << 24
	orderKiller1    = 1 << 20
	orderKiller2    = 1 << 19
	orderHistoryMax = 1 << 18

	// historyMax bounds a single history entry; reaching it halves the table.
	historyMax = 1 << 14
)

// orderMoves sorts moves in place into search order: the hash move first, then
// captures and promotions by MVV-LVA, then the two killers for this ply, then
// quiet moves by history score. The sort is stable, so moves with equal scores
// keep generation order. It never adds or drops a move; ordering changes the
// number of nodes visited but never the search score. A hash move that is not
// in the legal list is ignored.
func (s *searcher) orderMoves(b *Position, moves []Move, hashMove Move, ply int) []Move {
	if len(moves) < 2 {
		return moves
	}
	us := b.sideToMove()
	var killer1, killer2 Move
	if ply < len(s.killers) {
		killer1, killer2 = s.killers[ply][0], s.killers[ply][1]
	}
	var scores [256]int
	for i, m := range moves {
		scores[i] = s.moveScore(b, m, hashMove, killer1, killer2, us)
	}
	for i := 1; i < len(moves); i++ {
		move, score := moves[i], scores[i]
		j := i - 1
		for j >= 0 && scores[j] < score {
			moves[j+1], scores[j+1] = moves[j], scores[j]
			j--
		}
		moves[j+1], scores[j+1] = move, score
	}
	return moves
}

// moveScore returns the ordering score of one move. Captures come before any
// quiet move even when the history table is saturated, and the hash move comes
// before everything.
func (s *searcher) moveScore(b *Position, m, hashMove, killer1, killer2 Move, us Colour) int {
	if m == hashMove {
		return orderHashMove
	}
	if piece, promotion := m.Promotion(); promotion {
		return orderCapture + mvvLva(b, m) + pieceValues[piece]*16
	}
	if m.IsCapture() {
		return orderCapture + mvvLva(b, m)
	}
	if m == killer1 {
		return orderKiller1
	}
	if m == killer2 {
		return orderKiller2
	}
	score := s.history[us][m.From()][m.To()]
	if score > orderHistoryMax {
		score = orderHistoryMax
	}
	return score
}

// recordQuietCutoff updates the killer and history tables after a quiet move
// causes a beta cutoff. Captures are already ordered by MVV-LVA and are never
// recorded. History is halved when an entry saturates so old scores fade.
func (s *searcher) recordQuietCutoff(us Colour, m Move, depth, ply int) {
	if ply < len(s.killers) && s.killers[ply][0] != m {
		s.killers[ply][1] = s.killers[ply][0]
		s.killers[ply][0] = m
	}
	h := &s.history[us][m.From()][m.To()]
	if *h < historyMax {
		*h += depth * depth
	}
	if *h <= historyMax {
		return
	}
	for c := range s.history {
		for from := range s.history[c] {
			for to := range s.history[c][from] {
				s.history[c][from][to] /= 2
			}
		}
	}
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
