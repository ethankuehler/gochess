package chess

import (
	"iter"
)

// Square uses little-endian rank-file mapping: a1 = 0, h8 = 63.
type Square int

const squareCount = 64
const boardSize = 8

type FileRank struct {
	file uint64
	rank uint64
}

func FileRankFromSquare(square Square) FileRank {
	return FileRank{file: uint64(square % boardSize), rank: uint64(square / boardSize)}
}

func SquareFromFileRank(fr FileRank) Square {
	return Square(fr.file + fr.rank*boardSize)
}

func (fr FileRank) File() uint64 { return fr.file }

func (fr FileRank) Rank() uint64 { return fr.rank }

type Piece int

const (
	NoPiece Piece = -1
	Pawn    Piece = 0
	Knight  Piece = 1
	Bishop  Piece = 2
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

func (c Colour) Opposite() Colour {
	switch c {
	case White:
		return Black
	case Black:
		return White
	default:
		return Both
	}
}

const rank1Mask BitBoard = 255

const (
	fileAMask BitBoard = 0x0101010101010101
	fileHMask BitBoard = 0x8080808080808080
)

const (
	a1 Square = 0
	b1 Square = 1
	c1 Square = 2
	d1 Square = 3
	e1 Square = 4
	f1 Square = 5
	g1 Square = 6
	h1 Square = 7

	a8 Square = 56
	b8 Square = 57
	c8 Square = 58
	d8 Square = 59
	e8 Square = 60
	f8 Square = 61
	g8 Square = 62
	h8 Square = 63
)

// Symbol order must match the Piece constants and blackPieceOffset.
var (
	pieceSymbols        = []string{"P", "N", "B", "R", "Q", "K", "p", "n", "b", "r", "q", "k"}
	unicodePieceSymbols = []string{"♙", "♘", "♗", "♖", "♕", "♔", "♟", "♞", "♝", "♜", "♛", "♚"}
	castlingSymbols     = []string{"K", "Q", "k", "q"}
	fileNames           = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'}
)
