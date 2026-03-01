package chess

import (
	"iter"
)

type Shift int

const SHIFT_START = 1
const SHIFT_STOP = 64 + 1
const SHIFT_SIZE = 64
const RANK_FILE_SIZE = 8

type Coordinates struct {
	row uint64
	col uint64
}

func CoordsFromShift(shift Shift) Coordinates {
	col := shift % RANK_FILE_SIZE
	row := shift / RANK_FILE_SIZE
	return Coordinates{uint64(row), uint64(col)}
}

func ShiftFromCoords(coord Coordinates) Shift {
	return Shift(coord.col + coord.row*RANK_FILE_SIZE)
}

// index for certin pieces.
type Piece int

const (
	ALL    Piece = -1
	PAWN   Piece = 0
	BISHOP Piece = 1
	KNIGHT Piece = 2
	ROOK   Piece = 3
	QUEEN  Piece = 4
	KING   Piece = 5
)

const BLACK_OFFSET = 6

func PiecesIter(colour Colour) iter.Seq[Piece] {
	var start Piece
	var stop Piece
	switch colour {
	case WHITE:
		start = 0
		stop = BLACK_OFFSET
	case BLACK:
		start = BLACK_OFFSET
		stop = BLACK_OFFSET + 6
	case BOTH:
		start = 0
		stop = 12
	}

	return func(yield func(Piece) bool) {
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

type Colour int

const (
	BOTH  Colour = -1
	WHITE Colour = 0
	BLACK Colour = 1
)

// precalculated positional masking.
const (
	ROW_MASK    BitBoard = 255
	COLUMN_MASK BitBoard = 72340172838076673
)

const (
	KNIGHT_MASK   BitBoard = 43234889994
	KNIGHT_OFFSET BitBoard = 18
)

const (
	KING_MASK   BitBoard = 0
	KING_OFFSET BitBoard = 0
)

// Use display_binary.py to conferm these numbers
// Mask is going to be a bit mask
// Offset is always a shift number
const (
	WHITE_PAWN_MOVE_MASK_2   BitBoard = 65792
	WHITE_PAWN_MOVE_OFFSET_2 BitBoard = 0
	WHITE_PAWN_MOVE_MASK     BitBoard = 256
	WHITE_PAWN_MOVE_OFFSET   BitBoard = 0
	WHITE_PAWN_ATTACK_MASK   BitBoard = 1280
	WHITE_PAWN_ATTACK_OFFSET BitBoard = 1
	BLACK_PAWN_MOVE_MASK_2   BitBoard = 257
	BLACK_PAWN_MOVE_OFFSET_2 BitBoard = 16
	BLACK_PAWN_MOVE_MASK     BitBoard = 1
	BLACK_PAWN_MOVE_OFFSET   BitBoard = 8
	BLACK_PAWN_ATTACK_MASK   BitBoard = 5
	BLACK_PAWN_ATTACK_OFFSET BitBoard = 9
)

// string information for formating and chess notations.
var (
	PICECES_SYM     = []string{"P", "B", "N", "R", "Q", "K", "p", "b", "n", "r", "q", "k"}
	UNI_PICECES_SYM = []string{"♙", "♗", "♘", "♖", "♕", "♔", "♟", "♝", "♞", "♜", "♛", "♚"}
	CASTLE_SYM      = []string{"K", "Q", "k", "q"}
	COLUMNS         = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'}
)
