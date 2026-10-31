package chess

import (
	"slices"
	"strings"
)

type PerftResult struct {
	Move  Move
	Nodes uint64
}

func (b *Position) Perft(depth int) uint64 {
	ensureAttackTables()
	if depth < 0 {
		return 0
	}
	return b.perft(depth)
}

// A value receiver keeps the copy-make child on the stack.
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
