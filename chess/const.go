package chess

import (
	"iter"
)

type Square int

const squareCount = 64
const boardSize = 8

type FileRank struct {
	row uint64
	col uint64
}

func FileRankFromSquare(shift Square) FileRank {
	col := shift % boardSize
	row := shift / boardSize
	return FileRank{row: uint64(row), col: uint64(col)}
}

func SquareFromFileRank(coord FileRank) Square {
	return Square(coord.col + coord.row*boardSize)
}

// Index for certain pieces.
type Piece int

const (
	NoPiece Piece = -1
	Pawn    Piece = 0
	Bishop  Piece = 1
	Knight  Piece = 2
	Rook    Piece = 3
	Queen   Piece = 4
	King    Piece = 5
)

const blackPieceOffset = 6

func PieceTypes(colour Colour) iter.Seq[Piece] {
	var start Piece
	var stop Piece
	switch colour {
	case White:
		start = 0
		stop = blackPieceOffset
	case Black:
		start = blackPieceOffset
		stop = blackPieceOffset + 6
	case Both:
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
	whiteToMove            uint8 = 1
	whiteKingsideCastling  uint8 = 1 << 1
	whiteQueensideCastling uint8 = 1 << 2
	blackKingsideCastling  uint8 = 1 << 3
	blackQueensideCastling uint8 = 1 << 4
)

type Colour int

const (
	Both  Colour = -1
	White Colour = 0
	Black Colour = 1
)

// precalculated positional masking.
const (
	rank1Mask BitBoard = 255
	fileAMask BitBoard = 72340172838076673
)

// String information for formatting and chess notation.
var (
	pieceSymbols        = []string{"P", "B", "N", "R", "Q", "K", "p", "b", "n", "r", "q", "k"}
	unicodePieceSymbols = []string{"♙", "♗", "♘", "♖", "♕", "♔", "♟", "♝", "♞", "♜", "♛", "♚"}
	castlingSymbols     = []string{"K", "Q", "k", "q"}
	fileNames           = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'}
)
