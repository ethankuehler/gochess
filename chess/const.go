package chess

import (
	"iter"
)

type Shift int

const SHIFT_SIZE = 64
const ROW_COL_SIZE = 8

type Coordinates struct {
	row uint64
	col uint64
}

func CoordsFromShift(shift Shift) Coordinates {
	col := shift % ROW_COL_SIZE
	row := shift / ROW_COL_SIZE
	return Coordinates{row: uint64(row), col: uint64(col)}
}

func ShiftFromCoords(coord Coordinates) Shift {
	return Shift(coord.col + coord.row*ROW_COL_SIZE)
}

// Index for certain pieces.
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

// String information for formatting and chess notation.
var (
	pieceSymbols        = []string{"P", "B", "N", "R", "Q", "K", "p", "b", "n", "r", "q", "k"}
	unicodePieceSymbols = []string{"♙", "♗", "♘", "♖", "♕", "♔", "♟", "♝", "♞", "♜", "♛", "♚"}
	CASTLE_SYM          = []string{"K", "Q", "k", "q"}
	COLUMNS             = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'}
)
