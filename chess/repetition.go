package chess

import "math/bits"

// PositionKey identifies a position for threefold-repetition detection. Two
// positions have the same key when the same pieces occupy the same squares, the
// same side is to move, the same castling rights remain, and the same en passant
// capture is available. The move clocks are not part of the key.
type PositionKey uint64

// Key returns the repetition key for the position. It is maintained
// incrementally by makeMove and is therefore cheap to call in the search.
func (b *Position) Key() PositionKey { return PositionKey(b.key) }

// IsFiftyMoveDraw reports whether the halfmove clock has reached 100 plies,
// which makes the fifty-move rule claimable by the side to move.
func (b *Position) IsFiftyMoveDraw() bool { return b.halfmoveClock >= 100 }

// IsSeventyFiveMoveDraw reports whether the halfmove clock has reached 150
// plies. Unlike the fifty-move rule, the seventy-five-move rule is an
// automatic draw and does not require a claim.
func (b *Position) IsSeventyFiveMoveDraw() bool { return b.halfmoveClock >= 150 }

// IsInsufficientMaterial reports whether neither side can possibly deliver
// checkmate with the material on the board. It implements the standard simple
// dead-position test: no pawns, rooks, or queens, at most one minor piece, or
// only bishops confined to one colour complex.
func (b *Position) IsInsufficientMaterial() bool {
	for _, piece := range []Piece{Pawn, Rook, Queen} {
		if b.pieceBitboards[piece] != 0 || b.pieceBitboards[int(piece)+blackPieceOffset] != 0 {
			return false
		}
	}

	knights := b.pieceBitboards[Knight] | b.pieceBitboards[Knight+blackPieceOffset]
	bishops := b.pieceBitboards[Bishop] | b.pieceBitboards[Bishop+blackPieceOffset]
	minors := bits.OnesCount64(uint64(knights)) + bits.OnesCount64(uint64(bishops))
	if minors <= 1 {
		return true
	}
	// With knights present, checkmate may still be possible; with bishops on
	// both colour complexes it definitely is.
	if knights != 0 || bishops == 0 {
		return false
	}
	color := -1
	for bishops != 0 {
		square := Square(bits.TrailingZeros64(uint64(bishops)))
		bishops &= bishops - 1
		fr := FileRankFromSquare(square)
		squareColor := int(fr.file+fr.rank) & 1
		if color == -1 {
			color = squareColor
		} else if color != squareColor {
			return false
		}
	}
	return true
}

// RepetitionCount returns how many times this position's key occurs in history.
// history holds keys of earlier positions, oldest first.
func (b *Position) RepetitionCount(history []PositionKey) int {
	key := b.Key()
	count := 0
	for _, previous := range history {
		if previous == key {
			count++
		}
	}
	return count
}

// IsThreefoldRepetition reports whether the position has already occurred twice
// in history, making the current occurrence the third. The repeat does not have
// to be consecutive. A threefold repetition is claimable, not automatic; see
// IsFivefoldRepetition for the automatic version.
func (b *Position) IsThreefoldRepetition(history []PositionKey) bool {
	return b.RepetitionCount(history) >= 2
}

// IsFivefoldRepetition reports whether the position has already occurred four
// times in history, making the current occurrence the fifth. Fivefold
// repetition is an automatic draw and does not require a claim.
func (b *Position) IsFivefoldRepetition(history []PositionKey) bool {
	return b.RepetitionCount(history) >= 4
}

// computeKey hashes the position from scratch. It is used when a position is
// created from a FEN; makeMove updates the key incrementally instead.
func (b *Position) computeKey() uint64 {
	var key uint64
	for index, pieces := range b.pieceBitboards {
		for pieces != 0 {
			square := Square(bits.TrailingZeros64(uint64(pieces)))
			key ^= zobristPieces[index][square]
			pieces &= pieces - 1
		}
	}
	key ^= zobristCastling[(b.flags&^whiteToMove)>>1]
	key ^= b.computeEnPassantKey()
	if b.flags&whiteToMove == 0 {
		key ^= zobristSide
	}
	return key
}

// computeEnPassantKey returns the hash contribution of the en passant target.
// FIDE considers positions different only when the en passant capture is
// actually available, so a target with no adjacent pawn contributes nothing.
func (b *Position) computeEnPassantKey() uint64 {
	if !b.hasEnPassantCapture() {
		return 0
	}
	square := Square(bits.TrailingZeros64(uint64(b.enPassantSquare)))
	return zobristEnPassant[FileRankFromSquare(square).file]
}

// hasEnPassantCapture reports whether a pawn of the side to move is positioned
// to capture en passant. It checks only pseudo-legal availability, without
// asking whether the capture would expose the king.
func (b *Position) hasEnPassantCapture() bool {
	if b.enPassantSquare == 0 {
		return false
	}
	target := Square(bits.TrailingZeros64(uint64(b.enPassantSquare)))
	fr := FileRankFromSquare(target)

	if b.sideToMove() == White {
		if fr.rank != 5 {
			return false
		}
		pawns := b.pieceBitboards[Pawn]
		if fr.file > 0 && pawns&(BitBoard(1)<<(target-9)) != 0 {
			return true
		}
		if fr.file < 7 && pawns&(BitBoard(1)<<(target-7)) != 0 {
			return true
		}
		return false
	}

	if fr.rank != 2 {
		return false
	}
	pawns := b.pieceBitboards[Pawn+blackPieceOffset]
	if fr.file > 0 && pawns&(BitBoard(1)<<(target+7)) != 0 {
		return true
	}
	if fr.file < 7 && pawns&(BitBoard(1)<<(target+9)) != 0 {
		return true
	}
	return false
}

// Zobrist tables. The values are generated deterministically with splitmix64,
// so keys are reproducible within a process and across runs without storing a
// table on disk. Keys are only compared within one search, not persisted.
var (
	zobristPieces    [12][squareCount]uint64
	zobristCastling  [16]uint64
	zobristEnPassant [8]uint64
	zobristSide      uint64
)

func init() {
	state := uint64(0x9E3779B97F4A7C15)
	next := func() uint64 {
		state += 0x9E3779B97F4A7C15
		z := state
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		return z ^ (z >> 31)
	}

	for index := range zobristPieces {
		for square := range zobristPieces[index] {
			zobristPieces[index][square] = next()
		}
	}
	for index := range zobristCastling {
		zobristCastling[index] = next()
	}
	for index := range zobristEnPassant {
		zobristEnPassant[index] = next()
	}
	zobristSide = next()
}
