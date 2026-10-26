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

// attackPenalty is the centipawn penalty for a piece that is under threat: it
// is hanging (attacked and undefended) or a cheaper material class attacks it.
// The values are roughly ten percent of material value; the king is priced
// separately because an attacked king is a check rather than a capture threat.
// Evaluate is a zero-sum White-relative score, so the bonus for threatening an
// enemy piece is the same term seen from the other side. The weights stay well
// below the piece's value because the attack map ignores pins and a static test
// cannot resolve who wins a capture sequence.
var attackPenalty = [6]int{
	Pawn:   10,
	Knight: 30,
	Bishop: 30,
	Rook:   50,
	Queen:  80,
	King:   50,
}

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
// A positive score favours White. It sums the piece-square table, then adjusts
// for attacks on the pieces: a hanging piece, or a piece attacked by a cheaper
// piece, costs attackPenalty[piece]. A king is charged whenever it is attacked,
// because that is check. Evaluate requires the attack tables (BuildAllAttacks)
// and does not mutate the position.
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

// attackMaps describes the squares a colour attacks, with the pawn, minor, and
// rook parts broken out so the evaluator can compare attacker and target value
// classes. Queen and king attacks are part of all but are not cheaper attackers
// for any piece. Attacks from pinned pieces are included, matching the FIDE
// definition used by isSquareAttacked.
type attackMaps struct {
	all   BitBoard
	pawn  BitBoard
	minor BitBoard
	rook  BitBoard
}

// attackInfo returns colour's attack map with the current occupancy.
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

// attackedPieceScore returns the total penalty for colour's pieces that the
// opponent attacks. A piece is counted when it is undefended, or when a piece
// from a cheaper material class attacks it even if it is defended. Defending
// against an equal or more valuable attacker is left to the search, which can
// resolve the trade. A king is counted whenever it is attacked, because a check
// cannot be answered by defending the king's square.
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

// evaluateForSideToMove returns the static score from the side to move's point
// of view, which is the convention used by the search.
func (b *Position) evaluateForSideToMove() int {
	score := b.Evaluate()
	if b.sideToMove() == Black {
		score = -score
	}
	return score
}
