package chess

import (
	"context"
	"sync/atomic"
	"time"
)

// MateScore is adjusted by ply to prefer shorter mates.
const MateScore = 1_000_000

const (
	mateScore = MateScore
	maxPly    = 128
	infinity  = mateScore + 1

	searchCheckInterval = 256

	aspirationDelta      = 25
	aspirationMaxRetries = 4

	futilityMargin = 100

	maxRepeatHistory = 100
)

// SearchClock is safe to update while a search is running.
type SearchClock struct {
	soft atomic.Int64
	hard atomic.Int64
}

func (c *SearchClock) SetDeadlines(soft, hard time.Time) {
	c.soft.Store(deadlineNanos(soft))
	c.hard.Store(deadlineNanos(hard))
}

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

func (c *SearchClock) softPassed() bool {
	d := c.soft.Load()
	return d != 0 && time.Now().UnixNano() >= d
}

func (c *SearchClock) hardPassed() bool {
	d := c.hard.Load()
	return d != 0 && time.Now().UnixNano() >= d
}

type SearchLimits struct {
	MaxDepth       int
	MaxNodes       uint64
	Deadline       time.Time
	SoftDeadline   time.Time
	Clock          *SearchClock
	RootMoves      []Move // nil means every legal move
	GameHistory    []PositionKey
	TT             *TranspositionTable
	CollectMetrics bool
}

// SearchMetrics timings are inclusive and do not overlap.
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

type SearchInfo struct {
	Depth    int
	SelDepth int
	Score    int
	Nodes    uint64
	Time     time.Duration
	PV       []Move
}

type SearchResult struct {
	BestMove   Move
	PonderMove Move
	Score      int
	HasMove    bool
	Draw       bool
	Depth      int
	SelDepth   int
	Nodes      uint64
	PV         []Move
	TT         TTStats
	Metrics    *SearchMetrics
}

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

// SearchWithLimits returns the last completed iteration when stopped early.
func (b *Position) SearchWithLimits(ctx context.Context, limits SearchLimits, info func(SearchInfo)) SearchResult {
	return b.searchWithOptions(ctx, limits, info, defaultSearchOptions())
}

type searchOptions struct {
	pvs        bool
	aspiration bool
	nullMove   bool
	lmr        bool
	futility   bool
}

func defaultSearchOptions() searchOptions {
	return searchOptions{pvs: true, aspiration: true, nullMove: true, lmr: true, futility: true}
}

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
		// UCI still requires a legal move at a drawn root.
		result.BestMove = moves[0]
		result.Score = 0
		result.HasMove = true
		result.Draw = true
		result.PV = []Move{moves[0]}
		return result
	}
	s.keys[s.pathBase] = b.key

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

// searchRoot gives repetition moves a one-move PV.
func (s *searcher) searchRoot(b *Position, ordered []Move, depth, alpha, beta int) (best Move, score int, pv []Move, aborted bool) {
	score = -infinity
	wideWindow := true
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

func nearMate(score int) bool {
	return score >= mateScoreTTThreshold || score <= -mateScoreTTThreshold
}

func mateWindow(alpha, beta int) bool {
	return beta >= mateScoreTTThreshold || alpha <= -mateScoreTTThreshold
}

func (s *searcher) futilityAllowed(depth int, pv, inCheck bool, alpha, beta int) bool {
	return s.opts.futility && !pv && !inCheck && depth <= 2 && !mateWindow(alpha, beta)
}

func (s *searcher) futilityPrunes(m, hashMove Move, depth, staticEval, alpha int, haveStaticEval bool, best int) bool {
	if !haveStaticEval || best <= -infinity || m == hashMove {
		return false
	}
	if _, promotion := m.Promotion(); m.IsCapture() || promotion {
		return false
	}
	return staticEval+futilityMargin*depth <= alpha
}

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

func (s *searcher) nullMoveAllowed(b *Position, depth int, pv, inCheck, nullAllowed bool, alpha, beta int) bool {
	return s.opts.nullMove && !pv && !inCheck && nullAllowed && depth >= 5 &&
		!mateWindow(alpha, beta) && b.hasNonPawnMaterial(b.sideToMove())
}

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

	pathBase int
	keys     [maxRepeatHistory + maxPly + 1]uint64

	pvTable  [maxPly + 1][maxPly + 1]Move
	pvLength [maxPly + 1]int

	killers [maxPly + 1][2]Move
	history [2][64][64]int
}

// repeatsEarlier treats twofold repetition on the search path as a draw.
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
	s.keys[s.pathBase+ply] = b.key

	// Table cutoffs and unsound pruning only run at null-window nodes.
	pv := s.opts.pvs && beta-alpha > 1

	alphaOrig := alpha
	inCheck := b.InCheck()
	ttUsable := s.tt != nil && b.halfmoveClock < ttMaxHalfmoveClock
	var hashMove Move
	var staticEval int
	haveStaticEval := false

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
			reduction := 3
			child := s.makeNullMove(&b)
			score := -s.negamax(child, depth-1-reduction, -beta, -beta+1, ply+1, false)
			if s.stopped {
				return 0
			}
			if score >= beta {
				// A null-move cutoff does not prove a mate score.
				if score >= mateScoreTTThreshold {
					score = beta
				}
				if ttUsable {
					s.storeTT(b.key, depth, score, alpha, beta, ply, Move{})
				}
				return score
			}
		}
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
		return 0
	}

	// Checkmate takes precedence over draw claims.
	if inCheck {
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
		if s.futilityPrunes(m, hashMove, depth, staticEval, alpha, haveStaticEval, best) {
			first = false
			moveIndex++
			continue
		}
		// A repeated move is not searched and must not inherit a sibling's PV.
		if ply+1 < len(s.pvLength) {
			s.pvLength[ply+1] = 0
		}
		child := s.makeMove(&b, m)
		score := 0
		if s.repeatsEarlier(child.key, int(child.halfmoveClock), ply) {
			score = 0
		} else if !s.opts.pvs || (first && pv) {
			score = -s.negamax(child, depth-1, -beta, -alpha, ply+1, true)
		} else if reduction := s.lateMoveReduction(&child, m, depth, moveIndex, pv, alpha, beta); reduction > 0 {
			score = -s.negamax(child, depth-1-reduction, -alpha-1, -alpha, ply+1, true)
			if score > alpha {
				score = -s.negamax(child, depth-1, -beta, -alpha, ply+1, true)
			}
		} else {
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

// probeTT only cuts off null-window nodes.
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

const (
	orderHashMove   = 1 << 30
	orderCapture    = 1 << 24
	orderKiller1    = 1 << 20
	orderKiller2    = 1 << 19
	orderHistoryMax = 1 << 18

	historyMax = 1 << 14
)

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

// recordQuietCutoff ages history by halving it when an entry saturates.
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

// ScoreToMate returns a signed distance in moves.
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
