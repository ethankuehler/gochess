package chess

import "math/bits"

func (b *Position) sideToMove() Colour {
	if b.flags&whiteToMove != 0 {
		return White
	}
	return Black
}

func (b *Position) SideToMove() Colour { return b.sideToMove() }

func (b *Position) kingSquare(colour Colour) Square {
	var kings BitBoard
	if colour == White {
		kings = b.pieceBitboards[King]
	} else {
		kings = b.pieceBitboards[King+blackPieceOffset]
	}
	if kings == 0 {
		panic("kingSquare: king is missing")
	}
	return Square(bits.TrailingZeros64(uint64(kings)))
}

func (b *Position) pieceAt(square Square) (colour Colour, piece Piece, ok bool) {
	mask := BitBoard(1) << square
	for i, v := range b.pieceBitboards {
		if v&mask == 0 {
			continue
		}
		if i >= blackPieceOffset {
			return Black, Piece(i - blackPieceOffset), true
		}
		return White, Piece(i), true
	}
	return Both, NoPiece, false
}

func (b *Position) PieceAt(square Square) (colour Colour, piece Piece, ok bool) {
	return b.pieceAt(square)
}

var castlingClearTable = [squareCount]uint8{
	a1: whiteQueensideCastling,
	e1: whiteKingsideCastling | whiteQueensideCastling,
	h1: whiteKingsideCastling,
	a8: blackQueensideCastling,
	e8: blackKingsideCastling | blackQueensideCastling,
	h8: blackKingsideCastling,
}

func castlingRightsToClear(square Square) uint8 {
	return castlingClearTable[square]
}

// makeMove requires the flags supplied by move generation.
func (b *Position) makeMove(m Move) Position {
	return b.makeMoveInternal(m, true)
}

func (b *Position) nullMove() Position {
	next := *b
	next.key = b.key ^ b.epHash ^ zobristSide
	next.enPassantSquare = 0
	next.epHash = 0
	next.flags ^= whiteToMove
	if next.halfmoveClock < 65535 {
		next.halfmoveClock++
	}
	if b.sideToMove() == Black && next.fullmoveNumber < 65535 {
		next.fullmoveNumber++
	}
	return next
}

func (b *Position) hasNonPawnMaterial(colour Colour) bool {
	offset := Piece(0)
	if colour == Black {
		offset = blackPieceOffset
	}
	return b.pieceBitboards[Knight+offset]|b.pieceBitboards[Bishop+offset]|
		b.pieceBitboards[Rook+offset]|b.pieceBitboards[Queen+offset] != 0
}

// The result is only valid for king-safety checks, not as a game position.
func (b *Position) makeMoveForLegality(m Move) Position {
	return b.makeMoveInternal(m, false)
}

func (b *Position) makeMoveInternal(m Move, updateKey bool) Position {
	next := *b
	from, to := Square(m.from), Square(m.to)
	us := b.sideToMove()
	them := us.Opposite()

	offset := Piece(0)
	if us == Black {
		offset = blackPieceOffset
	}
	themOffset := Piece(0)
	if them == Black {
		themOffset = blackPieceOffset
	}

	fromMask := BitBoard(1) << from
	toMask := BitBoard(1) << to

	key := uint64(0)
	if updateKey {
		key = b.key
		key ^= zobristCastling[(b.flags&^whiteToMove)>>1]
		key ^= b.epHash
	}

	moving := NoPiece
	for piece := Pawn; piece <= King; piece++ {
		bb := &next.pieceBitboards[piece+offset]
		if *bb&fromMask != 0 {
			*bb &^= fromMask
			next.occupancy[us] &^= fromMask
			moving = piece
			if updateKey {
				key ^= zobristPieces[int(piece)+int(offset)][from]
			}
			break
		}
	}
	if moving == NoPiece {
		panic("makeMove: no piece on the from square")
	}

	if m.IsEnPassant() {
		captured := to - 8
		if us == Black {
			captured = to + 8
		}
		capturedMask := BitBoard(1) << captured
		if updateKey && next.pieceBitboards[Pawn+themOffset]&capturedMask != 0 {
			key ^= zobristPieces[int(Pawn)+int(themOffset)][captured]
		}
		next.pieceBitboards[Pawn+themOffset] &^= capturedMask
		next.occupancy[them] &^= capturedMask
	} else if m.IsCapture() {
		next.occupancy[them] &^= toMask
		for piece := Pawn; piece <= King; piece++ {
			if updateKey && next.pieceBitboards[piece+themOffset]&toMask != 0 {
				key ^= zobristPieces[int(piece)+int(themOffset)][to]
			}
			next.pieceBitboards[piece+themOffset] &^= toMask
		}
	}

	if m.flags&promotionFlag != 0 {
		promotion, ok := m.Promotion()
		if !ok || promotion < Knight || promotion > Queen {
			panic("makeMove: invalid promotion piece")
		}
		next.pieceBitboards[promotion+offset] |= toMask
		next.occupancy[us] |= toMask
		if updateKey {
			key ^= zobristPieces[int(promotion)+int(offset)][to]
		}
	} else {
		next.pieceBitboards[moving+offset] |= toMask
		next.occupancy[us] |= toMask
		if updateKey {
			key ^= zobristPieces[int(moving)+int(offset)][to]
		}
	}

	if m.IsCastling() {
		var rookFrom, rookTo Square
		switch {
		case us == White && to == g1:
			rookFrom, rookTo = h1, f1
		case us == White && to == c1:
			rookFrom, rookTo = a1, d1
		case us == Black && to == g8:
			rookFrom, rookTo = h8, f8
		case us == Black && to == c8:
			rookFrom, rookTo = a8, d8
		default:
			panic("makeMove: invalid castling move")
		}
		rooks := &next.pieceBitboards[Rook+offset]
		*rooks &^= BitBoard(1) << rookFrom
		*rooks |= BitBoard(1) << rookTo
		next.occupancy[us] &^= BitBoard(1) << rookFrom
		next.occupancy[us] |= BitBoard(1) << rookTo
		if updateKey {
			key ^= zobristPieces[int(Rook)+int(offset)][rookFrom]
			key ^= zobristPieces[int(Rook)+int(offset)][rookTo]
		}
	}

	if updateKey {
		next.flags &^= castlingRightsToClear(from) | castlingRightsToClear(to)

		next.enPassantSquare = 0
		next.epHash = 0
		if m.IsDoublePush() {
			next.enPassantSquare = BitBoard(1) << ((from + to) / 2)
			next.epHash = next.computeEnPassantKey()
		}

		if moving == Pawn || m.IsCapture() {
			next.halfmoveClock = 0
		} else if next.halfmoveClock < 65535 {
			next.halfmoveClock++
		}
		if us == Black && next.fullmoveNumber < 65535 {
			next.fullmoveNumber++
		}

		next.flags ^= whiteToMove

		key ^= zobristCastling[(next.flags&^whiteToMove)>>1]
		key ^= next.epHash
		key ^= zobristSide
		next.key = key
	}

	return next
}
