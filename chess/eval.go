package chess

import "math/bits"

// Evaluation scores are integer centipawns: +100 means White is up one pawn.
// Every table below is from White's point of view; black uses the vertically
// mirrored square and the negated value.

// pieceValues are the material values indexed by Piece.
var pieceValues = [6]int{
	Pawn: 100, Bishop: 330, Knight: 320, Rook: 500, Queen: 900, King: 0,
}

// centralityWeight is the centipawn value of one centrality step for each
// piece type. Kings and pawns are handled by their own terms.
var centralityWeight = [6]int{
	Pawn: 0, Bishop: 4, Knight: 5, Rook: 3, Queen: 4, King: 0,
}

// centrality runs from 0 on the rim to 5 in the four central squares. The
// table is written rank 1 first, matching square = file + rank*8.
var centrality = [64]int{
	0, 0, 1, 2, 2, 1, 0, 0, // rank 1
	0, 1, 2, 3, 3, 2, 1, 0, // rank 2
	1, 2, 3, 4, 4, 3, 2, 1, // rank 3
	2, 3, 4, 5, 5, 4, 3, 2, // rank 4
	2, 3, 4, 5, 5, 4, 3, 2, // rank 5
	1, 2, 3, 4, 4, 3, 2, 1, // rank 6
	0, 1, 2, 3, 3, 2, 1, 0, // rank 7
	0, 0, 1, 2, 2, 1, 0, 0, // rank 8
}

// kingSafety rewards a cornered or castled king and penalizes a king in the
// center. It is deliberately phase-blind: king activity in the endgame is a
// follow-up, not part of this table.
var kingSafety = [64]int{
	30, 10, 25, 5, 5, 10, 25, 30, // rank 1
	10, 5, 0, -5, -5, 0, 5, 10, // rank 2
	-5, -10, -15, -20, -20, -15, -10, -5, // rank 3
	-10, -15, -20, -25, -25, -20, -15, -10, // rank 4
	-10, -15, -20, -25, -25, -20, -15, -10, // rank 5
	-5, -10, -15, -20, -20, -15, -10, -5, // rank 6
	10, 5, 0, -5, -5, 0, 5, 10, // rank 7
	30, 10, 25, 5, 5, 10, 25, 30, // rank 8
}

// pawnAdvance is the bonus for a white pawn's zero-based rank. Black pawns use
// the mirrored rank, so a black pawn on rank 2 receives pawnAdvance[6].
var pawnAdvance = [8]int{0, 0, 5, 10, 20, 40, 80, 0}

// pieceSquareValues holds material plus every positional term for each piece
// and colour. Entries for black pieces are already negated, so Evaluate is a
// single sum over the twelve bitboards.
var pieceSquareValues = buildPieceSquareValues()

// buildPieceSquareValues builds the piece-square table from the term tables.
func buildPieceSquareValues() (table [12][64]int) {
	for piece := Pawn; piece <= King; piece++ {
		for square := Square(0); square < squareCount; square++ {
			table[piece][square] = pieceSquareScore(piece, square)
			mirrored := square ^ 56 // flip the rank
			table[piece+blackPieceOffset][square] = -pieceSquareScore(piece, mirrored)
		}
	}
	return table
}

// pieceSquareScore returns White's score for one white piece on square.
func pieceSquareScore(piece Piece, square Square) int {
	score := pieceValues[piece] + centralityWeight[piece]*centrality[square]
	switch piece {
	case King:
		score += kingSafety[square]
	case Pawn:
		score += pawnAdvance[FileRankFromSquare(square).Rank()]
	}
	return score
}

// Evaluate returns the static score in centipawns from White's point of view.
// A positive score favours White. Evaluate needs no attack tables and does not
// mutate the position.
func (b *Position) Evaluate() int {
	score := 0
	for index, board := range b.pieceBitboards {
		for board != 0 {
			square := Square(bits.TrailingZeros64(uint64(board)))
			board &= board - 1
			score += pieceSquareValues[index][square]
		}
	}
	return score
}

// evaluateForSideToMove returns the static score from the side to move's point
// of view, which is the convention used by the search.
func (b *Position) evaluateForSideToMove() int {
	score := b.Evaluate()
	if b.sideToMove() == Black {
		score = -score
	}
	return score
}
