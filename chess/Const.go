package chess

import (
	"iter"
)

// index for certin pieces.
const (
	ALL          = -1
	PAWN         = 0
	BISHOP       = 1
	KNIGHT       = 2
	ROOK         = 3
	QUEEN        = 4
	KING         = 5
	BLACK_OFFSET = 6
)

func PicecesIter(colour int64) iter.Seq[uint64] {
	var start uint64
	var stop uint64
	switch colour {
	case WHITE:
		start = 0
		stop = 6
	case BLACK:
		start = BLACK_OFFSET
		stop = BLACK_OFFSET + 6
	case BOTH:
		start = 0
		stop = 12
	}

	return func(yield func(uint64) bool) {
		for i := start; i < stop; i++ {
			if !yield(i) {
				return
			}
		}
	}

}

// mask for game encoding.
const (
	TURN_MASK     uint8 = 1
	WHITEOO_MASK  uint8 = 1 << 1
	WHITEOOO_MASK uint8 = 1 << 2
	BLACKOO_MASK  uint8 = 1 << 3
	BLACKOOO_MASK uint8 = 1 << 4
)

const (
	BOTH  = -1
	WHITE = 0
	BLACK = 1
)

// precalculated positional masking.
const (
	ROW_MASK    uint64 = 255
	COLUMN_MASK uint64 = 72340172838076673
)

const (
	KNIGHT_MASK   uint64 = 43234889994
	KNIGHT_OFFSET uint64 = 18
)

const (
	KING_MASK   uint64 = 0
	KING_OFFSET uint64 = 0
)

// Use display_binary.py to conferm these numbers
// Mask is going to be a bit mask
// Offset is always a shift number
const (
	WHITE_PAWN_MOVE_MASK_2   uint64 = 65792
	WHITE_PAWN_MOVE_OFFSET_2 uint64 = 0
	WHITE_PAWN_MOVE_MASK     uint64 = 256
	WHITE_PAWN_MOVE_OFFSET   uint64 = 0
	WHITE_PAWN_ATTACK_MASK   uint64 = 1280
	WHITE_PAWN_ATTACK_OFFSET uint64 = 1
	BLACK_PAWN_MOVE_MASK_2   uint64 = 257
	BLACK_PAWN_MOVE_OFFSET_2 uint64 = 16
	BLACK_PAWN_MOVE_MASK     uint64 = 1
	BLACK_PAWN_MOVE_OFFSET   uint64 = 8
	BLACK_PAWN_ATTACK_MASK   uint64 = 5
	BLACK_PAWN_ATTACK_OFFSET uint64 = 9
)

// string information for formating and chess notations.
var (
	PICECES_SYM     = []string{"P", "B", "N", "R", "Q", "K", "p", "b", "n", "r", "q", "k"}
	UNI_PICECES_SYM = []string{"♙", "♗", "♘", "♖", "♕", "♔", "♟", "♝", "♞", "♜", "♛", "♚"}
	CASTLE_SYM      = []string{"K", "Q", "k", "q"}
	COLUMNS         = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'}
)
