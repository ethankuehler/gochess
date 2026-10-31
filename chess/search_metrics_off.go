//go:build !metrics

package chess

const MetricsEnabled = false

func (s *searcher) legalMoves(b *Position, buf []Move) []Move {
	return b.legalMoves(buf)
}

func (s *searcher) evaluate(b *Position) int {
	return b.evaluateForSideToMove()
}

func (s *searcher) makeMove(b *Position, move Move) Position {
	return b.makeMove(move)
}

func (s *searcher) makeNullMove(b *Position) Position {
	return b.nullMove()
}
