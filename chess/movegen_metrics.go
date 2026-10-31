//go:build metrics

package chess

import (
	"math/bits"
	"time"
)

type moveGenStats struct {
	pseudoMoves   uint64
	legalMoves    uint64
	makeMoveCalls uint64
	generationNS  int64
	filterNS      int64
}

func (b *Position) strictlyLegalPawnMovesWithStats(moves []Move, ctx *legalContext, offset Piece, stats *moveGenStats) []Move {
	pushTable, attackTable := pawnMoveTables(ctx.us)
	for pawns := b.pieceBitboards[Pawn+offset]; pawns != 0; pawns &= pawns - 1 {
		from := Square(bits.TrailingZeros64(uint64(pawns)))
		before := len(moves)
		moves = strictlyLegalOrdinaryPawnMoves(moves, ctx, from, pushTable, attackTable)
		stats.pseudoMoves += uint64(len(moves) - before)

		if targets := attackTable[from] & b.enPassantSquare; targets != 0 {
			to := Square(bits.TrailingZeros64(uint64(targets)))
			m := newMove(from, to, captureFlag|enPassantFlag)
			stats.pseudoMoves++
			filterStart := time.Now()
			next := b.makeMoveForLegality(m)
			stats.makeMoveCalls++
			legal := !next.isSquareAttacked(next.kingSquare(ctx.us), ctx.them)
			stats.filterNS += time.Since(filterStart).Nanoseconds()
			if legal {
				moves = append(moves, m)
			}
		}
	}
	return moves
}

func (b *Position) addStrictlyLegalKingMovesWithStats(moves []Move, ctx *legalContext, stats *moveGenStats) []Move {
	targets := kingAttacks[ctx.king] &^ ctx.own
	for targets != 0 {
		to := Square(bits.TrailingZeros64(uint64(targets)))
		targets &= targets - 1
		m := newMove(ctx.king, to, 0)
		if ctx.enemy&(BitBoard(1)<<to) != 0 {
			m = newMove(ctx.king, to, captureFlag)
		}
		stats.pseudoMoves++
		filterStart := time.Now()
		next := b.makeMoveForLegality(m)
		stats.makeMoveCalls++
		legal := !next.isSquareAttacked(next.kingSquare(ctx.us), ctx.them)
		stats.filterNS += time.Since(filterStart).Nanoseconds()
		if legal {
			moves = append(moves, m)
		}
	}
	return moves
}

// Keep this in sync with strictlyLegalMoves.
func (b *Position) legalMovesWithStats(moves []Move, stats *moveGenStats) []Move {
	generationStart := time.Now()
	filterBefore := stats.filterNS
	initialLen := len(moves)
	ctx := b.newLegalContext()
	offset := Piece(0)
	if ctx.us == Black {
		offset = blackPieceOffset
	}

	if ctx.checkerCount < 2 {
		moves = b.strictlyLegalPawnMovesWithStats(moves, &ctx, offset, stats)
		before := len(moves)
		moves = b.strictlyLegalPieceMoves(moves, &ctx, offset)
		stats.pseudoMoves += uint64(len(moves) - before)
	}
	moves = b.addStrictlyLegalKingMovesWithStats(moves, &ctx, stats)
	if ctx.checkerCount == 0 {
		before := len(moves)
		moves = b.addCastlingMoves(moves, ctx.us, ctx.occupancy)
		stats.pseudoMoves += uint64(len(moves) - before)
	}

	stats.legalMoves += uint64(len(moves) - initialLen)
	generationNS := time.Since(generationStart).Nanoseconds() - (stats.filterNS - filterBefore)
	stats.generationNS += generationNS
	return moves
}
