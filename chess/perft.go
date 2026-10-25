package chess

import (
	"slices"
	"strings"
)

// PerftResult is one root move and the number of leaf nodes below it.
type PerftResult struct {
	Move  Move
	Nodes uint64
}

// Perft returns the number of leaf nodes at exactly depth. Perft(0) is 1 and
// Perft(1) is the number of legal moves. The attack tables must have been
// built with BuildAllAttacks or the individual builders.
func (b *Position) Perft(depth int) uint64 {
	ensureAttackTables()
	if depth < 0 {
		return 0
	}
	return b.perft(depth)
}

// perft takes Position by value. With a pointer receiver, taking the address of
// the copy-make child at each internal node makes it escape, so perft
// heap-allocates one Position per node; the value receiver keeps it on the
// stack and makes the whole walk allocation-free.
func (b Position) perft(depth int) uint64 {
	if depth == 0 {
		return 1
	}

	var buf [256]Move
	legal := b.legalMoves(buf[:0])
	if depth == 1 {
		return uint64(len(legal))
	}

	var nodes uint64
	for _, m := range legal {
		next := b.makeMove(m)
		nodes += next.perft(depth - 1)
	}
	return nodes
}

// PerftDivide returns the node count below each legal root move, sorted by
// UCI notation. It is the divide form used to compare against Stockfish.
func (b *Position) PerftDivide(depth int) []PerftResult {
	ensureAttackTables()
	if depth <= 0 {
		return nil
	}

	var buf [256]Move
	legal := b.legalMoves(buf[:0])
	results := make([]PerftResult, 0, len(legal))
	for _, m := range legal {
		var nodes uint64
		if depth == 1 {
			nodes = 1
		} else {
			next := b.makeMove(m)
			nodes = next.perft(depth - 1)
		}
		results = append(results, PerftResult{Move: m, Nodes: nodes})
	}

	slices.SortFunc(results, func(a, b PerftResult) int {
		return strings.Compare(a.Move.String(), b.Move.String())
	})
	return results
}
