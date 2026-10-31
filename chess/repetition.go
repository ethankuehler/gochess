package chess

import "math/bits"

// PositionKey excludes move clocks and en passant targets with no legal capture.
type PositionKey uint64

func (b *Position) Key() PositionKey { return PositionKey(b.key) }

func (b *Position) IsFiftyMoveDraw() bool { return b.halfmoveClock >= 100 }

func (b *Position) IsSeventyFiveMoveDraw() bool { return b.halfmoveClock >= 150 }

// IsInsufficientMaterial covers lone minors and bishops on one colour complex.
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

func (b *Position) IsThreefoldRepetition(history []PositionKey) bool {
	return b.RepetitionCount(history) >= 2
}

func (b *Position) IsFivefoldRepetition(history []PositionKey) bool {
	return b.RepetitionCount(history) >= 4
}

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

// FIDE only hashes an en passant target when a capture is available.
func (b *Position) computeEnPassantKey() uint64 {
	if !b.hasEnPassantCapture() {
		return 0
	}
	square := Square(bits.TrailingZeros64(uint64(b.enPassantSquare)))
	return zobristEnPassant[FileRankFromSquare(square).file]
}

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

// Deterministic splitmix64 keys need no stored table.
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
