//go:build metrics

package chess

import "time"

// MetricsEnabled reports whether this build compiles in search metrics.
const MetricsEnabled = true

func (s *searcher) legalMoves(b *Position, buf []Move) []Move {
	if s.metrics == nil {
		return b.legalMoves(buf)
	}
	var stats moveGenStats
	moves := b.legalMovesWithStats(buf, &stats)
	s.metrics.MoveGenCalls++
	s.metrics.PseudoMoves += stats.pseudoMoves
	s.metrics.LegalMoves += stats.legalMoves
	s.metrics.MakeMoveCalls += stats.makeMoveCalls
	s.metrics.MoveGenNS += stats.generationNS
	s.metrics.LegalFilterNS += stats.filterNS
	return moves
}

func (s *searcher) evaluate(b *Position) int {
	if s.metrics == nil {
		return b.evaluateForSideToMove()
	}
	start := time.Now()
	score := b.evaluateForSideToMove()
	s.metrics.EvaluationCalls++
	s.metrics.EvaluationNS += time.Since(start).Nanoseconds()
	return score
}

func (s *searcher) makeNullMove(b *Position) Position {
	if s.metrics == nil {
		return b.nullMove()
	}
	start := time.Now()
	child := b.nullMove()
	s.metrics.MakeMoveCalls++
	s.metrics.SearchMakeMoveCalls++
	s.metrics.SearchMakeMoveNS += time.Since(start).Nanoseconds()
	return child
}

func (s *searcher) makeMove(b *Position, move Move) Position {
	if s.metrics == nil {
		return b.makeMove(move)
	}
	start := time.Now()
	child := b.makeMove(move)
	s.metrics.MakeMoveCalls++
	s.metrics.SearchMakeMoveCalls++
	s.metrics.SearchMakeMoveNS += time.Since(start).Nanoseconds()
	return child
}
