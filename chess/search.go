package chess

// MateScore is the score of a checkmate at the root. Scores within 128 points of
// ±MateScore encode forced mates and describe the distance to mate.
const MateScore = 1_000_000

const (
	mateScore = MateScore
	maxPly    = 128
	infinity  = mateScore + 1
)

// SearchResult is the outcome of a fixed-depth alpha-beta search.
type SearchResult struct {
	BestMove Move   // valid only when HasMove is true
	Score    int    // centipawns from the side to move's point of view
	HasMove  bool   // false for checkmate and stalemate
	Depth    int    // search depth in plies
	Nodes    uint64 // negamax nodes visited, excluding the root
}

// Search returns the best move for the side to move at the given depth in plies.
// It panics if depth is less than 1. The attack tables must have been built with
// BuildAllAttacks or the individual builders.
func (b *Position) Search(depth int) SearchResult {
	if depth < 1 {
		panic("chess: search depth must be at least 1")
	}
	ensureAttackTables()

	result := SearchResult{Depth: depth}
	var buf, ordered [256]Move
	moves := b.legalMoves(buf[:0])
	if len(moves) == 0 {
		if b.InCheck() {
			result.Score = -mateScore
		}
		return result
	}

	s := searcher{}
	alpha, beta := -infinity, infinity
	best := -infinity
	for _, m := range orderMoves(moves, ordered[:0]) {
		child := b.makeMove(m)
		score := -s.negamax(child, depth-1, -beta, -alpha, 1)
		if score > best {
			best = score
			result.BestMove = m
			result.HasMove = true
		}
		if score > alpha {
			alpha = score
		}
	}
	result.Score = best
	result.Nodes = s.nodes
	return result
}

// searcher carries the node counter for one search.
type searcher struct {
	nodes uint64
}

// negamax returns the score of the position for the side to move. ply is the
// distance from the root and is used only to prefer shorter mates. Position is
// passed by value: it is small, and the value parameter keeps the copy-make child
// on the stack instead of forcing a heap allocation per node.
func (s *searcher) negamax(b Position, depth, alpha, beta, ply int) int {
	s.nodes++

	// The fifty-move rule makes the position a claimable draw.
	if b.halfmoveClock >= 100 {
		return 0
	}

	var buf, ordered [256]Move
	if depth == 0 {
		// Checkmate at the horizon still has to be detected, otherwise a
		// depth-1 search would miss mate in one.
		if b.InCheck() && len(b.legalMoves(buf[:0])) == 0 {
			return -mateScore + ply
		}
		return b.evaluateForSideToMove()
	}

	moves := b.legalMoves(buf[:0])
	if len(moves) == 0 {
		if b.InCheck() {
			return -mateScore + ply
		}
		return 0 // stalemate
	}

	best := -infinity
	for _, m := range orderMoves(moves, ordered[:0]) {
		child := b.makeMove(m)
		score := -s.negamax(child, depth-1, -beta, -alpha, ply+1)
		if score > best {
			best = score
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			break
		}
	}
	return best
}

// orderMoves puts captures, en passant, and promotions before quiet moves. It
// writes into scratch and returns the used slice; ordering changes the number of
// nodes visited but never the search score.
func orderMoves(moves, scratch []Move) []Move {
	out := scratch[:0]
	for _, m := range moves {
		if _, promotion := m.Promotion(); m.IsCapture() || promotion {
			out = append(out, m)
		}
	}
	for _, m := range moves {
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
