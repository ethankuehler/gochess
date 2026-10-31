package chess

import (
	"fmt"
	"math/bits"
)

func ensureAttackTables() {
	ok := len(knightAttacks) == squareCount &&
		len(kingAttacks) == squareCount &&
		len(whitePawnAttacks) == squareCount &&
		len(blackPawnAttacks) == squareCount &&
		len(whitePawnMoves) == squareCount &&
		len(blackPawnMoves) == squareCount &&
		len(rookMagicEntries) == squareCount &&
		len(bishopMagicEntries) == squareCount &&
		len(rookAttackTables) == squareCount &&
		len(bishopAttackTables) == squareCount
	if !ok {
		panic("chess: attack tables are not initialized; call BuildAllAttacks")
	}
}

func (b *Position) isSquareAttacked(square Square, by Colour) bool {
	return b.isSquareAttackedWithOccupancy(square, by, b.Occupancy(Both))
}

func (b *Position) isSquareAttackedWithOccupancy(square Square, by Colour, occupancy BitBoard) bool {
	offset := Piece(0)
	if by == Black {
		offset = blackPieceOffset
	}

	target := BitBoard(1) << square
	if by == White {
		pawns := b.pieceBitboards[Pawn]
		attacks := ((pawns << 7) &^ fileHMask) | ((pawns << 9) &^ fileAMask)
		if attacks&target != 0 {
			return true
		}
	} else {
		pawns := b.pieceBitboards[Pawn+blackPieceOffset]
		attacks := ((pawns >> 7) &^ fileAMask) | ((pawns >> 9) &^ fileHMask)
		if attacks&target != 0 {
			return true
		}
	}

	if knightAttacks[square]&b.pieceBitboards[Knight+offset] != 0 {
		return true
	}
	if kingAttacks[square]&b.pieceBitboards[King+offset] != 0 {
		return true
	}
	if BishopAttack(square, occupancy)&(b.pieceBitboards[Bishop+offset]|b.pieceBitboards[Queen+offset]) != 0 {
		return true
	}
	if RookAttack(square, occupancy)&(b.pieceBitboards[Rook+offset]|b.pieceBitboards[Queen+offset]) != 0 {
		return true
	}
	return false
}

const allSquares = ^BitBoard(0)

type legalContext struct {
	us           Colour
	them         Colour
	own          BitBoard
	enemy        BitBoard
	occupancy    BitBoard
	king         Square
	checkers     BitBoard
	checkerCount int
	checkTarget  BitBoard
	pinned       BitBoard
	pinCount     int
	pinEntries   [8]pinnedPiece
}

type pinnedPiece struct {
	square Square
	mask   BitBoard
}

func (ctx *legalContext) pinMask(from Square) BitBoard {
	if ctx.pinned&(BitBoard(1)<<from) == 0 {
		return allSquares
	}
	for i := 0; i < ctx.pinCount; i++ {
		if ctx.pinEntries[i].square == from {
			return ctx.pinEntries[i].mask
		}
	}
	return allSquares
}

func rayBetween(from, to Square) (BitBoard, bool) {
	fromFile, fromRank := int(from)%boardSize, int(from)/boardSize
	toFile, toRank := int(to)%boardSize, int(to)/boardSize
	fileDelta, rankDelta := toFile-fromFile, toRank-fromRank
	if fileDelta == 0 && rankDelta == 0 {
		return 0, false
	}
	if fileDelta != 0 && rankDelta != 0 {
		fileDistance, rankDistance := fileDelta, rankDelta
		if fileDistance < 0 {
			fileDistance = -fileDistance
		}
		if rankDistance < 0 {
			rankDistance = -rankDistance
		}
		if fileDistance != rankDistance {
			return 0, false
		}
	}

	fileStep, rankStep := 0, 0
	if fileDelta > 0 {
		fileStep = 1
	} else if fileDelta < 0 {
		fileStep = -1
	}
	if rankDelta > 0 {
		rankStep = 1
	} else if rankDelta < 0 {
		rankStep = -1
	}
	step := fileStep + rankStep*boardSize

	var between BitBoard
	for square := int(from) + step; square != int(to); square += step {
		between |= BitBoard(1) << Square(square)
	}
	return between, true
}

func (b *Position) newLegalContext() legalContext {
	ctx := legalContext{
		us:          b.sideToMove(),
		checkTarget: allSquares,
	}
	ctx.them = ctx.us.Opposite()
	ctx.own = b.Occupancy(ctx.us)
	ctx.enemy = b.Occupancy(ctx.them)
	ctx.occupancy = ctx.own | ctx.enemy
	ctx.king = b.kingSquare(ctx.us)

	themOffset := Piece(0)
	if ctx.us == White {
		themOffset = blackPieceOffset
	}

	kingMask := BitBoard(1) << ctx.king
	if ctx.us == White {
		pawnAttackers := ((kingMask << 7) &^ fileHMask) | ((kingMask << 9) &^ fileAMask)
		ctx.checkers = pawnAttackers & b.pieceBitboards[Pawn+themOffset]
	} else {
		pawnAttackers := ((kingMask >> 7) &^ fileAMask) | ((kingMask >> 9) &^ fileHMask)
		ctx.checkers = pawnAttackers & b.pieceBitboards[Pawn+themOffset]
	}
	ctx.checkers |= knightAttacks[ctx.king] & b.pieceBitboards[Knight+themOffset]
	ctx.checkers |= kingAttacks[ctx.king] & b.pieceBitboards[King+themOffset]
	ctx.checkers |= BishopAttack(ctx.king, ctx.occupancy) & (b.pieceBitboards[Bishop+themOffset] | b.pieceBitboards[Queen+themOffset])
	ctx.checkers |= RookAttack(ctx.king, ctx.occupancy) & (b.pieceBitboards[Rook+themOffset] | b.pieceBitboards[Queen+themOffset])
	ctx.checkerCount = bits.OnesCount64(uint64(ctx.checkers))

	enemyBishops := b.pieceBitboards[Bishop+themOffset] | b.pieceBitboards[Queen+themOffset]
	enemyRooks := b.pieceBitboards[Rook+themOffset] | b.pieceBitboards[Queen+themOffset]
	pinners := (BishopAttack(ctx.king, ctx.occupancy&^ctx.own) & enemyBishops) |
		(RookAttack(ctx.king, ctx.occupancy&^ctx.own) & enemyRooks)
	for pinners != 0 {
		pinner := Square(bits.TrailingZeros64(uint64(pinners)))
		pinners &= pinners - 1
		between, _ := rayBetween(ctx.king, pinner)
		pinned := between & ctx.own
		if pinned != 0 && pinned&(pinned-1) == 0 {
			pinnedSquare := Square(bits.TrailingZeros64(uint64(pinned)))
			ctx.pinEntries[ctx.pinCount] = pinnedPiece{square: pinnedSquare, mask: between | (BitBoard(1) << pinner)}
			ctx.pinCount++
			ctx.pinned |= pinned
		}
	}

	if ctx.checkerCount == 1 {
		checker := Square(bits.TrailingZeros64(uint64(ctx.checkers)))
		ctx.checkTarget = ctx.checkers
		checkerMask := BitBoard(1) << checker
		if checkerMask&(b.pieceBitboards[Bishop+themOffset]|b.pieceBitboards[Rook+themOffset]|b.pieceBitboards[Queen+themOffset]) != 0 {
			between, _ := rayBetween(ctx.king, checker)
			ctx.checkTarget |= between
		}
	}

	return ctx
}

func appendTargets(moves []Move, from Square, targets, enemy BitBoard) []Move {
	for targets != 0 {
		to := Square(bits.TrailingZeros64(uint64(targets)))
		targets &= targets - 1
		var flags uint16
		if enemy&(BitBoard(1)<<to) != 0 {
			flags = captureFlag
		}
		moves = append(moves, newMove(from, to, flags))
	}
	return moves
}

func appendPromotions(moves []Move, from, to Square, extraFlags uint16) []Move {
	promotions := [...]Piece{Queen, Rook, Bishop, Knight}
	for _, promotion := range promotions {
		moves = append(moves, newMove(from, to, extraFlags|promotionFlag|uint16(promotion)))
	}
	return moves
}

func isPromotionRank(colour Colour, square Square) bool {
	rank := FileRankFromSquare(square).rank
	if colour == White {
		return rank == 7
	}
	return rank == 0
}

func (b *Position) canCastle(colour Colour, kingSide bool, occ BitBoard) bool {
	var (
		kingFrom, rookFrom Square
		emptyMask          BitBoard
		transit            [2]Square
		rights             uint8
		them               Colour
	)

	if colour == White {
		kingFrom, them = e1, Black
		if kingSide {
			rights = whiteKingsideCastling
			rookFrom = h1
			emptyMask = BitBoard(1)<<f1 | BitBoard(1)<<g1
			transit = [2]Square{f1, g1}
		} else {
			rights = whiteQueensideCastling
			rookFrom = a1
			emptyMask = BitBoard(1)<<b1 | BitBoard(1)<<c1 | BitBoard(1)<<d1
			transit = [2]Square{d1, c1}
		}
	} else {
		kingFrom, them = e8, White
		if kingSide {
			rights = blackKingsideCastling
			rookFrom = h8
			emptyMask = BitBoard(1)<<f8 | BitBoard(1)<<g8
			transit = [2]Square{f8, g8}
		} else {
			rights = blackQueensideCastling
			rookFrom = a8
			emptyMask = BitBoard(1)<<b8 | BitBoard(1)<<c8 | BitBoard(1)<<d8
			transit = [2]Square{d8, c8}
		}
	}

	if b.flags&rights == 0 {
		return false
	}

	offset := Piece(0)
	if colour == Black {
		offset = blackPieceOffset
	}
	if b.pieceBitboards[King+offset]&(BitBoard(1)<<kingFrom) == 0 {
		return false
	}
	if b.pieceBitboards[Rook+offset]&(BitBoard(1)<<rookFrom) == 0 {
		return false
	}
	if occ&emptyMask != 0 {
		return false
	}
	if b.isSquareAttacked(kingFrom, them) {
		return false
	}

	kingRemoved := occ &^ (BitBoard(1) << kingFrom)
	for _, square := range transit {
		if b.isSquareAttackedWithOccupancy(square, them, kingRemoved) {
			return false
		}
	}
	return true
}

// addCastlingMoves appends the legal castling moves for us.
func (b *Position) addCastlingMoves(moves []Move, us Colour, occ BitBoard) []Move {
	from := e1
	if us == Black {
		from = e8
	}

	if b.canCastle(us, true, occ) {
		to := g1
		if us == Black {
			to = g8
		}
		moves = append(moves, newMove(from, to, castlingFlag))
	}
	if b.canCastle(us, false, occ) {
		to := c1
		if us == Black {
			to = c8
		}
		moves = append(moves, newMove(from, to, castlingFlag))
	}
	return moves
}

func (b *Position) addStrictlyLegalKingMoves(moves []Move, ctx *legalContext) []Move {
	targets := kingAttacks[ctx.king] &^ ctx.own
	for targets != 0 {
		to := Square(bits.TrailingZeros64(uint64(targets)))
		targets &= targets - 1
		m := newMove(ctx.king, to, 0)
		if ctx.enemy&(BitBoard(1)<<to) != 0 {
			m = newMove(ctx.king, to, captureFlag)
		}
		next := b.makeMoveForLegality(m)
		if !next.isSquareAttacked(next.kingSquare(ctx.us), ctx.them) {
			moves = append(moves, m)
		}
	}
	return moves
}

func pawnMoveTables(colour Colour) (pushTable, attackTable []BitBoard) {
	if colour == White {
		return whitePawnMoves, whitePawnAttacks
	}
	return blackPawnMoves, blackPawnAttacks
}

func strictlyLegalOrdinaryPawnMoves(moves []Move, ctx *legalContext, from Square, pushTable, attackTable []BitBoard) []Move {
	allowed := ctx.checkTarget & ctx.pinMask(from)
	for pushes := pushTable[from] &^ ctx.occupancy & allowed; pushes != 0; pushes &= pushes - 1 {
		to := Square(bits.TrailingZeros64(uint64(pushes)))
		double := (ctx.us == White && to == from+16) || (ctx.us == Black && to == from-16)
		if double {
			middle := Square((int(from) + int(to)) / 2)
			if ctx.occupancy&(BitBoard(1)<<middle) != 0 {
				continue
			}
		}
		if isPromotionRank(ctx.us, to) {
			moves = appendPromotions(moves, from, to, 0)
			continue
		}
		var flags uint16
		if double {
			flags = doublePushFlag
		}
		moves = append(moves, newMove(from, to, flags))
	}

	for captures := attackTable[from] & ctx.enemy & allowed; captures != 0; captures &= captures - 1 {
		to := Square(bits.TrailingZeros64(uint64(captures)))
		if isPromotionRank(ctx.us, to) {
			moves = appendPromotions(moves, from, to, captureFlag)
			continue
		}
		moves = append(moves, newMove(from, to, captureFlag))
	}
	return moves
}

func (b *Position) strictlyLegalPawnMoves(moves []Move, ctx *legalContext, offset Piece) []Move {
	pushTable, attackTable := pawnMoveTables(ctx.us)
	for pawns := b.pieceBitboards[Pawn+offset]; pawns != 0; pawns &= pawns - 1 {
		from := Square(bits.TrailingZeros64(uint64(pawns)))
		moves = strictlyLegalOrdinaryPawnMoves(moves, ctx, from, pushTable, attackTable)
		if targets := attackTable[from] & b.enPassantSquare; targets != 0 {
			to := Square(bits.TrailingZeros64(uint64(targets)))
			m := newMove(from, to, captureFlag|enPassantFlag)
			next := b.makeMoveForLegality(m)
			if !next.isSquareAttacked(next.kingSquare(ctx.us), ctx.them) {
				moves = append(moves, m)
			}
		}
	}
	return moves
}

func (b *Position) strictlyLegalPieceMoves(moves []Move, ctx *legalContext, offset Piece) []Move {
	for pieces := b.pieceBitboards[Knight+offset]; pieces != 0; pieces &= pieces - 1 {
		from := Square(bits.TrailingZeros64(uint64(pieces)))
		targets := knightAttacks[from] &^ ctx.own & ctx.checkTarget & ctx.pinMask(from)
		moves = appendTargets(moves, from, targets, ctx.enemy)
	}
	for pieces := b.pieceBitboards[Bishop+offset]; pieces != 0; pieces &= pieces - 1 {
		from := Square(bits.TrailingZeros64(uint64(pieces)))
		targets := BishopAttack(from, ctx.occupancy) &^ ctx.own & ctx.checkTarget & ctx.pinMask(from)
		moves = appendTargets(moves, from, targets, ctx.enemy)
	}
	for pieces := b.pieceBitboards[Rook+offset]; pieces != 0; pieces &= pieces - 1 {
		from := Square(bits.TrailingZeros64(uint64(pieces)))
		targets := RookAttack(from, ctx.occupancy) &^ ctx.own & ctx.checkTarget & ctx.pinMask(from)
		moves = appendTargets(moves, from, targets, ctx.enemy)
	}
	for pieces := b.pieceBitboards[Queen+offset]; pieces != 0; pieces &= pieces - 1 {
		from := Square(bits.TrailingZeros64(uint64(pieces)))
		targets := QueenAttack(from, ctx.occupancy) &^ ctx.own & ctx.checkTarget & ctx.pinMask(from)
		moves = appendTargets(moves, from, targets, ctx.enemy)
	}
	return moves
}

func (b *Position) strictlyLegalMoves(moves []Move) []Move {
	ctx := b.newLegalContext()
	offset := Piece(0)
	if ctx.us == Black {
		offset = blackPieceOffset
	}

	if ctx.checkerCount < 2 {
		moves = b.strictlyLegalPawnMoves(moves, &ctx, offset)
		moves = b.strictlyLegalPieceMoves(moves, &ctx, offset)
	}
	moves = b.addStrictlyLegalKingMoves(moves, &ctx)
	if ctx.checkerCount == 0 {
		moves = b.addCastlingMoves(moves, ctx.us, ctx.occupancy)
	}
	return moves
}

func (b *Position) legalMoves(moves []Move) []Move {
	return b.strictlyLegalMoves(moves)
}

func (b *Position) LegalMoves() []Move {
	ensureAttackTables()
	var buf [256]Move
	return append([]Move(nil), b.legalMoves(buf[:0])...)
}

func (b *Position) InCheck() bool {
	ensureAttackTables()
	us := b.sideToMove()
	return b.isSquareAttacked(b.kingSquare(us), us.Opposite())
}

func (b *Position) ApplyMove(m Move) (*Position, error) {
	ensureAttackTables()
	var buf [256]Move
	for _, legal := range b.legalMoves(buf[:0]) {
		if legal == m {
			next := b.makeMove(m)
			return &next, nil
		}
	}
	return nil, fmt.Errorf("illegal move %s", m)
}
