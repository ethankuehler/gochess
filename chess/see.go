package chess

import "math/bits"

const seeKingValue = 10_000

func seePieceValue(piece Piece) int {
	if piece == King {
		return seeKingValue
	}
	return pieceValues[piece]
}

func seeAttackers(square Square, colour Colour, boards *[12]BitBoard, occupancy BitBoard) BitBoard {
	offset := Piece(0)
	if colour == Black {
		offset = blackPieceOffset
	}

	// Derive pawn origins with shifts, not the reverse attack tables: those
	// tables are zero on the ranks a pawn of that colour cannot occupy, but a
	// back-rank square still needs its 7th-rank pawn defenders (e.g. c7 defends
	// b8, and the recapture promotes).
	pawns := boards[Pawn+offset]
	target := BitBoard(1) << square
	var origins BitBoard
	if colour == White {
		origins = ((target >> 7) &^ fileAMask) | ((target >> 9) &^ fileHMask)
	} else {
		origins = ((target << 7) &^ fileHMask) | ((target << 9) &^ fileAMask)
	}
	attackers := origins & pawns
	attackers |= knightAttacks[square] & boards[Knight+offset]
	attackers |= kingAttacks[square] & boards[King+offset]
	attackers |= BishopAttack(square, occupancy) & (boards[Bishop+offset] | boards[Queen+offset])
	attackers |= RookAttack(square, occupancy) & (boards[Rook+offset] | boards[Queen+offset])
	return attackers
}

func leastValuableAttacker(attackers BitBoard, colour Colour, boards *[12]BitBoard) (Piece, Square) {
	offset := Piece(0)
	if colour == Black {
		offset = blackPieceOffset
	}
	for piece := Pawn; piece <= King; piece++ {
		pieces := attackers & boards[piece+offset]
		if pieces != 0 {
			return piece, Square(bits.TrailingZeros64(uint64(pieces)))
		}
	}
	return NoPiece, 0
}

func (b *Position) see(m Move) int {
	from, to := m.From(), m.To()
	us := b.sideToMove()
	them := us.Opposite()
	usOffset := Piece(0)
	themOffset := Piece(0)
	if us == Black {
		usOffset = blackPieceOffset
	} else {
		themOffset = blackPieceOffset
	}

	_, moving, _ := b.pieceAt(from)
	boards := b.pieceBitboards
	occupancy := b.Occupancy(Both)
	fromMask := BitBoard(1) << from
	toMask := BitBoard(1) << to
	boards[moving+usOffset] &^= fromMask
	occupancy &^= fromMask

	var gain [32]int
	captured := NoPiece
	capturedSquare := to
	if m.IsEnPassant() {
		captured = Pawn
		capturedSquare = to - 8
		if us == Black {
			capturedSquare = to + 8
		}
	} else if m.IsCapture() {
		_, captured, _ = b.pieceAt(to)
	}
	if captured != NoPiece {
		capturedMask := BitBoard(1) << capturedSquare
		boards[captured+themOffset] &^= capturedMask
		occupancy &^= capturedMask
		gain[0] = seePieceValue(captured)
	}

	onTarget := moving
	occupant := moving + usOffset
	if promotion, ok := m.Promotion(); ok {
		gain[0] += seePieceValue(promotion) - seePieceValue(Pawn)
		onTarget = promotion
		occupant = promotion + usOffset
	}
	boards[occupant] |= toMask
	occupancy |= toMask

	side := them
	depth := 0
	for {
		attackers := seeAttackers(to, side, &boards, occupancy)
		if attackers == 0 {
			break
		}
		attacker, attackerSquare := leastValuableAttacker(attackers, side, &boards)
		depth++
		gain[depth] = seePieceValue(onTarget) - gain[depth-1]

		offset := Piece(0)
		if side == Black {
			offset = blackPieceOffset
		}
		attackerMask := BitBoard(1) << attackerSquare
		boards[occupant] &^= toMask
		boards[attacker+offset] &^= attackerMask
		occupancy &^= attackerMask
		if attacker == Pawn && isPromotionRank(side, to) {
			// A recapture on the last rank promotes; count the gain and let the
			// promoted piece defend itself against a further recapture.
			gain[depth] += seePieceValue(Queen) - seePieceValue(Pawn)
			onTarget = Queen
			occupant = Queen + offset
		} else {
			onTarget = attacker
			occupant = attacker + offset
		}
		boards[occupant] |= toMask
		side = side.Opposite()
	}

	for depth > 0 {
		gain[depth-1] = -max(-gain[depth-1], gain[depth])
		depth--
	}
	return gain[0]
}
