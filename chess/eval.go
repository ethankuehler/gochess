package chess

import "math/bits"

// Evaluation is White-relative; black uses mirrored squares and negated values.
var pieceValues = [6]int{
	Pawn: 100, Bishop: 330, Knight: 320, Rook: 500, Queen: 900, King: 0,
}

var centralityWeight = [6]int{
	Pawn: 0, Bishop: 4, Knight: 5, Rook: 3, Queen: 4, King: 0,
}

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

var pawnAdvance = [8]int{0, 0, 5, 10, 20, 40, 80, 0}

var attackPenalty = [6]int{
	Pawn:   10,
	Knight: 30,
	Bishop: 30,
	Rook:   50,
	Queen:  80,
	King:   50,
}

var pieceSquareValues = buildPieceSquareValues()

func buildPieceSquareValues() (table [12][64]int) {
	for piece := Pawn; piece <= King; piece++ {
		for square := Square(0); square < squareCount; square++ {
			table[piece][square] = pieceSquareScore(piece, square)
			mirrored := square ^ 56
			table[piece+blackPieceOffset][square] = -pieceSquareScore(piece, mirrored)
		}
	}
	return table
}

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

// Evaluate returns centipawns from White's point of view.
func (b *Position) Evaluate() int {
	ensureAttackTables()
	score := 0
	for index, board := range b.pieceBitboards {
		for board != 0 {
			square := Square(bits.TrailingZeros64(uint64(board)))
			board &= board - 1
			score += pieceSquareValues[index][square]
		}
	}
	whiteAttacks := b.attackInfo(White)
	blackAttacks := b.attackInfo(Black)
	score -= b.attackedPieceScore(blackAttacks, whiteAttacks.all, White)
	score += b.attackedPieceScore(whiteAttacks, blackAttacks.all, Black)
	return score
}

type attackMaps struct {
	all   BitBoard
	pawn  BitBoard
	minor BitBoard
	rook  BitBoard
}

func (b *Position) attackInfo(colour Colour) attackMaps {
	offset := Piece(0)
	if colour == Black {
		offset = blackPieceOffset
	}

	var info attackMaps
	pawns := b.pieceBitboards[Pawn+offset]
	if colour == White {
		info.pawn = ((pawns << 7) &^ fileHMask) | ((pawns << 9) &^ fileAMask)
	} else {
		info.pawn = ((pawns >> 7) &^ fileAMask) | ((pawns >> 9) &^ fileHMask)
	}
	info.all = info.pawn
	info.all |= kingAttacks[b.kingSquare(colour)]

	for knights := b.pieceBitboards[Knight+offset]; knights != 0; knights &= knights - 1 {
		square := Square(bits.TrailingZeros64(uint64(knights)))
		info.minor |= knightAttacks[square]
	}

	occupancy := b.Occupancy(Both)
	for bishops := b.pieceBitboards[Bishop+offset]; bishops != 0; bishops &= bishops - 1 {
		square := Square(bits.TrailingZeros64(uint64(bishops)))
		info.minor |= BishopAttack(square, occupancy)
	}
	info.all |= info.minor

	for rooks := b.pieceBitboards[Rook+offset]; rooks != 0; rooks &= rooks - 1 {
		square := Square(bits.TrailingZeros64(uint64(rooks)))
		info.rook |= RookAttack(square, occupancy)
	}
	info.all |= info.rook

	for queens := b.pieceBitboards[Queen+offset]; queens != 0; queens &= queens - 1 {
		square := Square(bits.TrailingZeros64(uint64(queens)))
		info.all |= BishopAttack(square, occupancy) | RookAttack(square, occupancy)
	}
	return info
}

// Penalize hanging pieces and attacks by cheaper pieces; checks always count.
func (b *Position) attackedPieceScore(enemy attackMaps, defended BitBoard, colour Colour) int {
	offset := Piece(0)
	if colour == Black {
		offset = blackPieceOffset
	}

	free := enemy.all &^ defended
	danger := [6]BitBoard{
		Pawn:   free,
		Knight: free | enemy.pawn,
		Bishop: free | enemy.pawn,
		Rook:   free | enemy.pawn | enemy.minor,
		Queen:  free | enemy.pawn | enemy.minor | enemy.rook,
		King:   enemy.all,
	}
	score := 0
	for piece := Pawn; piece <= King; piece++ {
		count := bits.OnesCount64(uint64(danger[piece] & b.pieceBitboards[piece+offset]))
		score += attackPenalty[piece] * count
	}
	return score
}

func (b *Position) evaluateForSideToMove() int {
	score := b.Evaluate()
	if b.sideToMove() == Black {
		score = -score
	}
	return score
}
