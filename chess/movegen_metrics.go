//go:build metrics

package chess

import "time"

// moveGenStats captures instrumentation for one instrumented legal move
// generation.
type moveGenStats struct {
	pseudoMoves   uint64
	legalMoves    uint64
	makeMoveCalls uint64
	generationNS  int64
	filterNS      int64
}

// legalMovesWithStats is legalMoves with instrumentation. Pseudo-legal
// generation and legality filtering are timed separately. Every copy made while
// filtering is counted, including copies of moves that turn out to be illegal.
// stats must be non-nil. Keep this loop in sync with legalMoves in movegen.go.
func (b *Position) legalMovesWithStats(moves []Move, stats *moveGenStats) []Move {
	generationStart := time.Now()
	pseudo := b.pseudoLegalMoves(moves)
	stats.generationNS += time.Since(generationStart).Nanoseconds()
	stats.pseudoMoves += uint64(len(pseudo))

	us := b.sideToMove()
	them := us.Opposite()

	filterStart := time.Now()
	legal := pseudo[:0]
	for _, m := range pseudo {
		next := b.makeMoveForLegality(m)
		stats.makeMoveCalls++
		if !next.isSquareAttacked(next.kingSquare(us), them) {
			legal = append(legal, m)
		}
	}
	stats.filterNS += time.Since(filterStart).Nanoseconds()
	stats.legalMoves += uint64(len(legal))
	return legal
}
