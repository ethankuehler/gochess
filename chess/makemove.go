package chess

import "math/bits"

// sideToMove returns the colour whose turn it is.
func (b *Position) sideToMove() Colour {
	if b.flags&whiteToMove != 0 {
		return White
	}
	return Black
}

// SideToMove returns the colour whose turn it is.
func (b *Position) SideToMove() Colour { return b.sideToMove() }

// kingSquare returns the square of colour's king.
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

// pieceAt returns the piece on square. ok is false when the square is empty.
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

// PieceAt returns the piece on square. ok is false when the square is empty.
func (b *Position) PieceAt(square Square) (colour Colour, piece Piece, ok bool) {
	return b.pieceAt(square)
}

// castlingRightsToClear returns the rights that a move from or to square
// invalidates. Any move involving a king or rook home square clears the
// corresponding right, which also handles a rook being captured in place.
func castlingRightsToClear(square Square) uint8 {
	switch square {
	case e1:
		return whiteKingsideCastling | whiteQueensideCastling
	case a1:
		return whiteQueensideCastling
	case h1:
		return whiteKingsideCastling
	case e8:
		return blackKingsideCastling | blackQueensideCastling
	case a8:
		return blackQueensideCastling
	case h8:
		return blackKingsideCastling
	default:
		return 0
	}
}

// makeMove applies m and returns the resulting position. It copies the
// position rather than mutating it. m must come from move generation or
// otherwise carry the correct flags; a UCI move parsed by ParseUCIMove does
// not have enough information on its own.
func (b *Position) makeMove(m Move) Position {
	next := *b
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

	fromMask := BitBoard(1) << m.from
	toMask := BitBoard(1) << m.to

	// Start from the old key and remove the state that is about to change.
	key := b.key
	key ^= zobristCastling[(b.flags&^whiteToMove)>>1]
	key ^= b.enPassantKey()

	// Find and lift the moving piece.
	moving := NoPiece
	for piece := Pawn; piece <= King; piece++ {
		bb := &next.pieceBitboards[piece+offset]
		if *bb&fromMask != 0 {
			*bb &^= fromMask
			moving = piece
			key ^= zobristPieces[int(piece)+int(offset)][m.from]
			break
		}
	}
	if moving == NoPiece {
		panic("makeMove: no piece on the from square")
	}

	// Remove the captured piece, or the pawn captured en passant.
	if m.IsEnPassant() {
		captured := Square(int(m.to) - 8)
		if us == Black {
			captured = Square(int(m.to) + 8)
		}
		capturedMask := BitBoard(1) << captured
		if next.pieceBitboards[Pawn+themOffset]&capturedMask != 0 {
			key ^= zobristPieces[int(Pawn)+int(themOffset)][captured]
		}
		next.pieceBitboards[Pawn+themOffset] &^= capturedMask
	} else if m.IsCapture() {
		for piece := Pawn; piece <= King; piece++ {
			if next.pieceBitboards[piece+themOffset]&toMask != 0 {
				key ^= zobristPieces[int(piece)+int(themOffset)][m.to]
			}
			next.pieceBitboards[piece+themOffset] &^= toMask
		}
	}

	// Place the moving piece, replacing it with the promotion piece if needed.
	if m.flags&promotionFlag != 0 {
		promotion, ok := m.Promotion()
		if !ok || promotion < Knight || promotion > Queen {
			panic("makeMove: invalid promotion piece")
		}
		next.pieceBitboards[promotion+offset] |= toMask
		key ^= zobristPieces[int(promotion)+int(offset)][m.to]
	} else {
		next.pieceBitboards[moving+offset] |= toMask
		key ^= zobristPieces[int(moving)+int(offset)][m.to]
	}

	// Move the rook when castling.
	if m.IsCastling() {
		var rookFrom, rookTo Square
		switch {
		case us == White && m.to == g1:
			rookFrom, rookTo = h1, f1
		case us == White && m.to == c1:
			rookFrom, rookTo = a1, d1
		case us == Black && m.to == g8:
			rookFrom, rookTo = h8, f8
		case us == Black && m.to == c8:
			rookFrom, rookTo = a8, d8
		default:
			panic("makeMove: invalid castling move")
		}
		rooks := &next.pieceBitboards[Rook+offset]
		*rooks &^= BitBoard(1) << rookFrom
		*rooks |= BitBoard(1) << rookTo
		key ^= zobristPieces[int(Rook)+int(offset)][rookFrom]
		key ^= zobristPieces[int(Rook)+int(offset)][rookTo]
	}

	next.flags &^= castlingRightsToClear(m.from) | castlingRightsToClear(m.to)

	next.enPassantSquare = 0
	if m.IsDoublePush() {
		next.enPassantSquare = BitBoard(1) << Square((int(m.from)+int(m.to))/2)
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

	// Add the new castling rights, en passant target, and side to move.
	key ^= zobristCastling[(next.flags&^whiteToMove)>>1]
	key ^= next.enPassantKey()
	key ^= zobristSide
	next.key = key

	return next
}
